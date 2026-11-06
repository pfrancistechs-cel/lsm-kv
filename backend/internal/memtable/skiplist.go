package memtable

import (
	"bytes"
	"math/rand"
	"sync"
	"time"
)

const (
	DefaultMaxLevel = 16
	DefaultP        = 0.5
)

type Entry struct {
	Key         []byte
	Value       []byte
	IsTombstone bool
	Timestamp   int64
}

type Node struct {
	Entry   Entry
	Forward []*Node
}

type SkipList struct {
	mu        sync.RWMutex
	maxLevel  int
	p         float64
	level     int
	head      *Node
	sizeBytes int64
	count     int
	rnd       *rand.Rand
}

func NewSkipList(maxLevel int) *SkipList {
	if maxLevel <= 0 {
		maxLevel = DefaultMaxLevel
	}
	head := &Node{
		Forward: make([]*Node, maxLevel),
	}
	return &SkipList{
		maxLevel: maxLevel,
		p:        DefaultP,
		level:    1,
		head:     head,
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *SkipList) randomLevel() int {
	lvl := 1
	for s.rnd.Float64() < s.p && lvl < s.maxLevel {
		lvl++
	}
	return lvl
}

func (s *SkipList) Put(key, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	update := make([]*Node, s.maxLevel)
	current := s.head

	for i := s.level - 1; i >= 0; i-- {
		for current.Forward[i] != nil && bytes.Compare(current.Forward[i].Entry.Key, key) < 0 {
			current = current.Forward[i]
		}
		update[i] = current
	}

	current = current.Forward[0]

	// Key already exists, update entry
	if current != nil && bytes.Equal(current.Entry.Key, key) {
		oldSize := int64(len(current.Entry.Value))
		current.Entry.Value = value
		current.Entry.IsTombstone = false
		current.Entry.Timestamp = time.Now().UnixNano()
		s.sizeBytes += int64(len(value)) - oldSize
		return
	}

	// Insert new node
	lvl := s.randomLevel()
	if lvl > s.level {
		for i := s.level; i < lvl; i++ {
			update[i] = s.head
		}
		s.level = lvl
	}

	newNode := &Node{
		Entry: Entry{
			Key:         key,
			Value:       value,
			IsTombstone: false,
			Timestamp:   time.Now().UnixNano(),
		},
		Forward: make([]*Node, lvl),
	}

	for i := 0; i < lvl; i++ {
		newNode.Forward[i] = update[i].Forward[i]
		update[i].Forward[i] = newNode
	}

	s.sizeBytes += int64(len(key) + len(value) + 16)
	s.count++
}

func (s *SkipList) Delete(key []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	update := make([]*Node, s.maxLevel)
	current := s.head

	for i := s.level - 1; i >= 0; i-- {
		for current.Forward[i] != nil && bytes.Compare(current.Forward[i].Entry.Key, key) < 0 {
			current = current.Forward[i]
		}
		update[i] = current
	}

	current = current.Forward[0]

	if current != nil && bytes.Equal(current.Entry.Key, key) {
		current.Entry.IsTombstone = true
		current.Entry.Value = nil
		current.Entry.Timestamp = time.Now().UnixNano()
		return
	}

	// Insert tombstone if not present
	lvl := s.randomLevel()
	if lvl > s.level {
		for i := s.level; i < lvl; i++ {
			update[i] = s.head
		}
		s.level = lvl
	}

	newNode := &Node{
		Entry: Entry{
			Key:         key,
			Value:       nil,
			IsTombstone: true,
			Timestamp:   time.Now().UnixNano(),
		},
		Forward: make([]*Node, lvl),
	}

	for i := 0; i < lvl; i++ {
		newNode.Forward[i] = update[i].Forward[i]
		update[i].Forward[i] = newNode
	}

	s.sizeBytes += int64(len(key) + 16)
	s.count++
}

func (s *SkipList) Get(key []byte) (value []byte, isTombstone bool, found bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current := s.head
	for i := s.level - 1; i >= 0; i-- {
		for current.Forward[i] != nil && bytes.Compare(current.Forward[i].Entry.Key, key) < 0 {
			current = current.Forward[i]
		}
	}

	current = current.Forward[0]
	if current != nil && bytes.Equal(current.Entry.Key, key) {
		return current.Entry.Value, current.Entry.IsTombstone, true
	}
	return nil, false, false
}

func (s *SkipList) AllEntries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]Entry, 0, s.count)
	current := s.head.Forward[0]
	for current != nil {
		entries = append(entries, current.Entry)
		current = current.Forward[0]
	}
	return entries
}

func (s *SkipList) SizeBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sizeBytes
}

func (s *SkipList) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}


<!-- // Level probability p=0.25 tuned for optimal memory overhead
 -->

<!-- // Level probability p=0.25 -->
