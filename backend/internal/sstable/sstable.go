package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/memtable"
)

type SSTable struct {
	mu          sync.RWMutex
	ID          string
	Level       int
	Path        string
	file        *os.File
	BloomFilter *BloomFilter
	Index       *SparseIndex
	KeyCount    int
	SizeBytes   int64
}

const IndexInterval = 8 // Create an index entry every 8 records

func BuildFromEntries(id string, level int, path string, entries []memtable.Entry) (*SSTable, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	bf := NewBloomFilter(len(entries), 0.01)
	sparseIdx := NewSparseIndex()

	var currentOffset int64 = 0
	var blockStartOffset int64 = 0
	var blockKey []byte

	for i, entry := range entries {
		bf.Add(entry.Key)

		// Record index boundary
		if i%IndexInterval == 0 {
			if i > 0 {
				blockSize := int32(currentOffset - blockStartOffset)
				sparseIdx.Add(blockKey, blockStartOffset, blockSize)
			}
			blockStartOffset = currentOffset
			blockKey = entry.Key
		}

		keyLen := uint32(len(entry.Key))
		valLen := uint32(len(entry.Value))
		tombstone := byte(0)
		if entry.IsTombstone {
			tombstone = 1
		}

		record := make([]byte, 4+4+1+len(entry.Key)+len(entry.Value))
		binary.BigEndian.PutUint32(record[0:4], keyLen)
		binary.BigEndian.PutUint32(record[4:8], valLen)
		record[8] = tombstone
		copy(record[9:9+len(entry.Key)], entry.Key)
		copy(record[9+len(entry.Key):], entry.Value)

		n, err := file.Write(record)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		currentOffset += int64(n)
	}

	if len(entries) > 0 {
		blockSize := int32(currentOffset - blockStartOffset)
		sparseIdx.Add(blockKey, blockStartOffset, blockSize)
	}

	_ = file.Sync()

	return &SSTable{
		ID:          id,
		Level:       level,
		Path:        path,
		file:        file,
		BloomFilter: bf,
		Index:       sparseIdx,
		KeyCount:    len(entries),
		SizeBytes:   currentOffset,
	}, nil
}

func (s *SSTable) Get(key []byte) (value []byte, isTombstone bool, found bool, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Fast path: Bloom filter check
	if !s.BloomFilter.MayContain(key) {
		return nil, false, false, nil // Definite negative in O(1)
	}

	// 2. Binary search over in-memory sparse index
	offset, length, ok := s.Index.FindBlock(key)
	if !ok {
		return nil, false, false, nil
	}

	// 3. Read matched disk block
	blockBuf := make([]byte, length)
	if _, err := s.file.ReadAt(blockBuf, offset); err != nil && err != io.EOF {
		return nil, false, false, err
	}

	// Scan through records in block
	reader := bytes.NewReader(blockBuf)
	header := make([]byte, 9)

	for {
		_, err := io.ReadFull(reader, header)
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, false, false, err
		}

		keyLen := binary.BigEndian.Uint32(header[0:4])
		valLen := binary.BigEndian.Uint32(header[4:8])
		isTomb := header[8] == 1

		recordKey := make([]byte, keyLen)
		if _, err := io.ReadFull(reader, recordKey); err != nil {
			return nil, false, false, err
		}

		recordVal := make([]byte, valLen)
		if _, err := io.ReadFull(reader, recordVal); err != nil {
			return nil, false, false, err
		}

		if bytes.Equal(recordKey, key) {
			return recordVal, isTomb, true, nil
		}
	}

	return nil, false, false, nil
}

func (s *SSTable) AllEntries() ([]memtable.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	entries := make([]memtable.Entry, 0, s.KeyCount)
	header := make([]byte, 9)

	for {
		_, err := io.ReadFull(s.file, header)
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		keyLen := binary.BigEndian.Uint32(header[0:4])
		valLen := binary.BigEndian.Uint32(header[4:8])
		isTomb := header[8] == 1

		k := make([]byte, keyLen)
		if _, err := io.ReadFull(s.file, k); err != nil {
			return nil, err
		}

		v := make([]byte, valLen)
		if _, err := io.ReadFull(s.file, v); err != nil {
			return nil, err
		}

		entries = append(entries, memtable.Entry{
			Key:         k,
			Value:       v,
			IsTombstone: isTomb,
		})
	}

	return entries, nil
}

func (s *SSTable) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

