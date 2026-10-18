package memtable

import (
	"fmt"
	"testing"
)

func TestSkipListPutGet(t *testing.T) {
	sl := NewSkipList()
	sl.Put([]byte("key1"), []byte("val1"))
	val, ok := sl.Get([]byte("key1"))
	if !ok || string(val) != "val1" {
		t.Fatalf("expected val1, got %s", string(val))
	}
}

func TestSkipListDelete(t *testing.T) {
	sl := NewSkipList()
	sl.Put([]byte("key2"), []byte("val2"))
	sl.Delete([]byte("key2"))
	_, ok := sl.Get([]byte("key2"))
	if ok {
		t.Fatalf("expected key2 to be deleted")
	}
}

func BenchmarkSkipListPut(b *testing.B) {
	sl := NewSkipList()
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("k-%08d", i))
		sl.Put(key, []byte("benchmark-value-payload"))
	}
}
