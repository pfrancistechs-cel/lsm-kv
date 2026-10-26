package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/compaction"
	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/memtable"
	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/sstable"
	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/wal"
)

type Config struct {
	DataDir             string
	MemTableThreshold   int64
	MaxLevel0SSTables   int
}

type EngineStats struct {
	MemTableBytes     int64             `json:"memtable_bytes"`
	MemTableKeys      int               `json:"memtable_keys"`
	Level0TablesCount int               `json:"level0_tables_count"`
	Level1TablesCount int               `json:"level1_tables_count"`
	TotalDiskBytes    int64             `json:"total_disk_bytes"`
	BloomFilterHits   int64             `json:"bloom_filter_hits"`
	BloomFilterMisses int64             `json:"bloom_filter_misses"`
	TotalReads        int64             `json:"total_reads"`
	TotalWrites       int64             `json:"total_writes"`
	Tables            []SSTableMetadata `json:"tables"`
}

type SSTableMetadata struct {
	ID        string `json:"id"`
	Level     int    `json:"level"`
	KeyCount  int    `json:"key_count"`
	SizeBytes int64  `json:"size_bytes"`
}

type Engine struct {
	mu                sync.RWMutex
	cfg               Config
	memTable          *memtable.SkipList
	immMemTable       *memtable.SkipList
	wal               *wal.WAL
	compactor         *compaction.Compactor
	tables            map[int][]*sstable.SSTable
	totalReads        int64
	totalWrites       int64
	bloomHits         int64
	bloomMisses       int64
}

func Open(cfg Config) (*Engine, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.MemTableThreshold <= 0 {
		cfg.MemTableThreshold = 256 * 1024 // 256KB for easy flush triggering & visual demo
	}
	if cfg.MaxLevel0SSTables <= 0 {
		cfg.MaxLevel0SSTables = 4
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, err
	}

	walPath := filepath.Join(cfg.DataDir, "active.wal")
	w, err := wal.Open(walPath)
	if err != nil {
		return nil, err
	}

	mem := memtable.NewSkipList(16)
	// Replay WAL on startup to restore un-flushed writes
	_ = w.Replay(mem)

	e := &Engine{
		cfg:       cfg,
		memTable:  mem,
		wal:       w,
		compactor: compaction.NewCompactor(cfg.DataDir),
		tables: map[int][]*sstable.SSTable{
			0: make([]*sstable.SSTable, 0),
			1: make([]*sstable.SSTable, 0),
		},
	}

	return e, nil
}

func (e *Engine) Put(key, val []byte) error {
	atomic.AddInt64(&e.totalWrites, 1)

	// 1. Write to WAL
	if err := e.wal.Append(key, val, false); err != nil {
		return err
	}

	// 2. Put into MemTable
	e.memTable.Put(key, val)

	// 3. Flush check
	if e.memTable.SizeBytes() >= e.cfg.MemTableThreshold {
		_ = e.Flush()
	}

	return nil
}

func (e *Engine) Delete(key []byte) error {
	atomic.AddInt64(&e.totalWrites, 1)

	if err := e.wal.Append(key, nil, true); err != nil {
		return err
	}
	e.memTable.Delete(key)

	if e.memTable.SizeBytes() >= e.cfg.MemTableThreshold {
		_ = e.Flush()
	}
	return nil
}

func (e *Engine) Get(key []byte) ([]byte, bool, error) {
	atomic.AddInt64(&e.totalReads, 1)

	// 1. Check Active MemTable
	val, isTombstone, found := e.memTable.Get(key)
	if found {
		if isTombstone {
			return nil, false, nil
		}
		return val, true, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// 2. Check Immutable MemTable if flushing
	if e.immMemTable != nil {
		val, isTombstone, found := e.immMemTable.Get(key)
		if found {
			if isTombstone {
				return nil, false, nil
			}
			return val, true, nil
		}
	}

	// 3. Check Level 0 SSTables (in reverse order, newest first)
	for i := len(e.tables[0]) - 1; i >= 0; i-- {
		t := e.tables[0][i]
		if !t.BloomFilter.MayContain(key) {
			atomic.AddInt64(&e.bloomMisses, 1)
			continue
		}
		atomic.AddInt64(&e.bloomHits, 1)

		val, isTomb, found, err := t.Get(key)
		if err != nil {
			return nil, false, err
		}
		if found {
			if isTomb {
				return nil, false, nil
			}
			return val, true, nil
		}
	}

	// 4. Check Level 1 SSTables
	for i := len(e.tables[1]) - 1; i >= 0; i-- {
		t := e.tables[1][i]
		if !t.BloomFilter.MayContain(key) {
			atomic.AddInt64(&e.bloomMisses, 1)
			continue
		}
		atomic.AddInt64(&e.bloomHits, 1)

		val, isTomb, found, err := t.Get(key)
		if err != nil {
			return nil, false, err
		}
		if found {
			if isTomb {
				return nil, false, nil
			}
			return val, true, nil
		}
	}

	return nil, false, nil
}

func (e *Engine) Flush() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.memTable.Count() == 0 {
		return nil
	}

	entries := e.memTable.AllEntries()
	sstID := fmt.Sprintf("sst_lvl0_%d", time.Now().UnixNano())
	sstPath := filepath.Join(e.cfg.DataDir, sstID+".sst")

	newTable, err := sstable.BuildFromEntries(sstID, 0, sstPath, entries)
	if err != nil {
		return err
	}

	e.tables[0] = append(e.tables[0], newTable)

	// Reset memtable and rotate WAL
	e.memTable = memtable.NewSkipList(16)
	_ = e.wal.Truncate()

	// Trigger compaction if Level 0 exceeds quota
	if len(e.tables[0]) >= e.cfg.MaxLevel0SSTables {
		go func() {
			_ = e.TriggerCompaction()
		}()
	}

	return nil
}

func (e *Engine) TriggerCompaction() error {
	e.mu.Lock()
	if len(e.tables[0]) < 2 {
		e.mu.Unlock()
		return nil
	}

	tablesToCompact := make([]*sstable.SSTable, len(e.tables[0]))
	copy(tablesToCompact, e.tables[0])
	e.tables[0] = make([]*sstable.SSTable, 0)
	e.mu.Unlock()

	mergedTable, err := e.compactor.MergeSSTables(tablesToCompact, 1)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.tables[1] = append(e.tables[1], mergedTable)
	e.mu.Unlock()

	return nil
}

func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalDiskBytes int64 = 0
	tableMetas := make([]SSTableMetadata, 0)

	for lvl, tbls := range e.tables {
		for _, t := range tbls {
			totalDiskBytes += t.SizeBytes
			tableMetas = append(tableMetas, SSTableMetadata{
				ID:        t.ID,
				Level:     lvl,
				KeyCount:  t.KeyCount,
				SizeBytes: t.SizeBytes,
			})
		}
	}

	return EngineStats{
		MemTableBytes:     e.memTable.SizeBytes(),
		MemTableKeys:      e.memTable.Count(),
		Level0TablesCount: len(e.tables[0]),
		Level1TablesCount: len(e.tables[1]),
		TotalDiskBytes:    totalDiskBytes,
		BloomFilterHits:   atomic.LoadInt64(&e.bloomHits),
		BloomFilterMisses: atomic.LoadInt64(&e.bloomMisses),
		TotalReads:        atomic.LoadInt64(&e.totalReads),
		TotalWrites:       atomic.LoadInt64(&e.totalWrites),
		Tables:            tableMetas,
	}
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	_ = e.wal.Close()
	for _, tbls := range e.tables {
		for _, t := range tbls {
			_ = t.Close()
		}
	}
	return nil
}

