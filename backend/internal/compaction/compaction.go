package compaction

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/memtable"
	"github.com/pfrancistechs-cel/lsm-kv/backend/internal/sstable"
)

type Compactor struct {
	dataDir string
}

func NewCompactor(dataDir string) *Compactor {
	return &Compactor{dataDir: dataDir}
}

// MergeSSTables takes multiple SSTables, performs a k-way sorted merge, eliminates superseded keys & tombstones, and outputs a merged SSTable
func (c *Compactor) MergeSSTables(tables []*sstable.SSTable, targetLevel int) (*sstable.SSTable, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("no sstables provided for compaction")
	}

	keyMap := make(map[string]memtable.Entry)

	// In reverse chronological order: later tables take precedence
	for _, table := range tables {
		entries, err := table.AllEntries()
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			keyStr := string(e.Key)
			if _, exists := keyMap[keyStr]; !exists {
				keyMap[keyStr] = e
			}
		}
	}

	mergedEntries := make([]memtable.Entry, 0, len(keyMap))
	for _, entry := range keyMap {
		// Only keep non-tombstones in highest target levels, or keep them if lower
		if !entry.IsTombstone || targetLevel == 0 {
			mergedEntries = append(mergedEntries, entry)
		}
	}

	sort.Slice(mergedEntries, func(i, j int) bool {
		return bytes.Compare(mergedEntries[i].Key, mergedEntries[j].Key) < 0
	})

	newID := fmt.Sprintf("sst_lvl%d_%d", targetLevel, time.Now().UnixNano())
	newPath := filepath.Join(c.dataDir, newID+".sst")

	newTable, err := sstable.BuildFromEntries(newID, targetLevel, newPath, mergedEntries)
	if err != nil {
		return nil, err
	}

	// Clean up old files
	for _, t := range tables {
		_ = t.Close()
		_ = os.Remove(t.Path)
	}

	return newTable, nil
}

