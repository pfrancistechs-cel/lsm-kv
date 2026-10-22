package sstable

import (
	"hash/fnv"
)

type BloomFilter struct {
	bits []byte
	size uint32
	k    uint32 // number of hash functions
}

func NewBloomFilter(expectedElements int, falsePositiveRate float64) *BloomFilter {
	if expectedElements <= 0 {
		expectedElements = 1000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	// m = - (n * ln(p)) / (ln(2)^2)
	// k = (m/n) * ln(2)
	m := uint32(float64(expectedElements) * 10.0) // ~10 bits per element for ~1% false positive
	if m < 64 {
		m = 64
	}
	bytesCount := (m + 7) / 8

	return &BloomFilter{
		bits: make([]byte, bytesCount),
		size: bytesCount * 8,
		k:    4,
	}
}

func hashPair(data []byte) (uint64, uint64) {
	h1 := fnv.New64a()
	_, _ = h1.Write(data)
	sum1 := h1.Sum64()

	h2 := fnv.New64()
	_, _ = h2.Write(data)
	sum2 := h2.Sum64()

	return sum1, sum2
}

func (bf *BloomFilter) Add(key []byte) {
	h1, h2 := hashPair(key)
	for i := uint32(0); i < bf.k; i++ {
		combined := (h1 + uint64(i)*h2) % uint64(bf.size)
		byteIndex := combined / 8
		bitIndex := combined % 8
		bf.bits[byteIndex] |= (1 << bitIndex)
	}
}

func (bf *BloomFilter) MayContain(key []byte) bool {
	h1, h2 := hashPair(key)
	for i := uint32(0); i < bf.k; i++ {
		combined := (h1 + uint64(i)*h2) % uint64(bf.size)
		byteIndex := combined / 8
		bitIndex := combined % 8
		if (bf.bits[byteIndex] & (1 << bitIndex)) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) Bytes() []byte {
	return bf.bits
}

func FromBytes(data []byte, k uint32) *BloomFilter {
	return &BloomFilter{
		bits: data,
		size: uint32(len(data) * 8),
		k:    k,
	}
}

