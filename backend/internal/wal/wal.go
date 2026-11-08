package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"sync"

	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/memtable"
)

type WAL struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func Open(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{
		path: path,
		file: file,
	}, nil
}

func (w *WAL) Append(key, val []byte, isTombstone bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	keyLen := uint32(len(key))
	valLen := uint32(len(val))
	tombstoneByte := byte(0)
	if isTombstone {
		tombstoneByte = byte(1)
	}

	payload := make([]byte, 4+4+1+len(key)+len(val))
	binary.BigEndian.PutUint32(payload[0:4], keyLen)
	binary.BigEndian.PutUint32(payload[4:8], valLen)
	payload[8] = tombstoneByte
	copy(payload[9:9+len(key)], key)
	copy(payload[9+len(key):], val)

	checksum := crc32.ChecksumIEEE(payload)
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, checksum)

	if _, err := w.file.Write(header); err != nil {
		return err
	}
	if _, err := w.file.Write(payload); err != nil {
		return err
	}

	return w.file.Sync()
}

func (w *WAL) Replay(mem *memtable.SkipList) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	header := make([]byte, 4)
	meta := make([]byte, 9)

	for {
		_, err := io.ReadFull(w.file, header)
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return err
		}

		expectedChecksum := binary.BigEndian.Uint32(header)

		if _, err := io.ReadFull(w.file, meta); err != nil {
			return err
		}

		keyLen := binary.BigEndian.Uint32(meta[0:4])
		valLen := binary.BigEndian.Uint32(meta[4:8])
		isTombstone := meta[8] == 1

		body := make([]byte, keyLen+valLen)
		if _, err := io.ReadFull(w.file, body); err != nil {
			return err
		}

		fullPayload := append(meta, body...)
		actualChecksum := crc32.ChecksumIEEE(fullPayload)
		if actualChecksum != expectedChecksum {
			return errors.New("wal record checksum mismatch: data corruption detected")
		}

		key := body[:keyLen]
		val := body[keyLen:]

		if isTombstone {
			mem.Delete(key)
		} else {
			mem.Put(key, val)
		}
	}

	return nil
}

func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_ = w.file.Close()
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	w.file = file
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}


<!-- // Batch fsync interval -->
