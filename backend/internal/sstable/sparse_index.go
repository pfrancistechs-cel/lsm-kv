package sstable

import "bytes"

type IndexEntry struct {
	Key    []byte
	Offset int64
	Length int32
}

type SparseIndex struct {
	Entries []IndexEntry
}

func NewSparseIndex() *SparseIndex {
	return &SparseIndex{
		Entries: make([]IndexEntry, 0),
	}
}

func (si *SparseIndex) Add(key []byte, offset int64, length int32) {
	si.Entries = append(si.Entries, IndexEntry{
		Key:    key,
		Offset: offset,
		Length: length,
	})
}

// FindBlock locates the offset of the block that could contain the search key
func (si *SparseIndex) FindBlock(key []byte) (int64, int32, bool) {
	if len(si.Entries) == 0 {
		return 0, 0, false
	}

	low := 0
	high := len(si.Entries) - 1
	candidateIdx := -1

	for low <= high {
		mid := low + (high-low)/2
		cmp := bytes.Compare(si.Entries[mid].Key, key)
		if cmp <= 0 {
			candidateIdx = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if candidateIdx == -1 {
		return 0, 0, false
	}

	return si.Entries[candidateIdx].Offset, si.Entries[candidateIdx].Length, true
}

