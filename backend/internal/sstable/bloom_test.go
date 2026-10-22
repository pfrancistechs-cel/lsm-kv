package sstable

import "testing"

func TestBloomFilter(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	bf.Add([]byte("alpha"))
	bf.Add([]byte("beta"))

	if !bf.MayContain([]byte("alpha")) {
		t.Errorf("expected alpha to be present")
	}
	if !bf.MayContain([]byte("beta")) {
		t.Errorf("expected beta to be present")
	}
}
