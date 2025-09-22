package protocol

import (
	"fmt"
	"sync"
	"testing"

	"github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
)

func TestFilterAll(t *testing.T) {
	tests := []struct {
		filter   *FilterNode
		tags     map[string]string
		expected bool
		desc     string
	}{
		// --- Leaf EQ ---
		{&FilterNode{Cmp: FilterCompareEQ, Key: "env", Val: "prod"}, map[string]string{"env": "prod"}, true, "EQ matches"},
		{&FilterNode{Cmp: FilterCompareEQ, Key: "env", Val: "prod"}, map[string]string{"env": "staging"}, false, "EQ does not match"},
		{&FilterNode{Cmp: FilterCompareEQ, Key: "env", Val: "prod"}, map[string]string{}, false, "EQ missing key"},

		// --- Leaf NOT_EQ ---
		{&FilterNode{Cmp: FilterCompareNotEQ, Key: "tier", Val: "bronze"}, map[string]string{"tier": "silver"}, true, "NOT_EQ different value"},
		{&FilterNode{Cmp: FilterCompareNotEQ, Key: "tier", Val: "bronze"}, map[string]string{"tier": "bronze"}, false, "NOT_EQ same value"},
		{&FilterNode{Cmp: FilterCompareNotEQ, Key: "tier", Val: "bronze"}, map[string]string{}, true, "NOT_EQ missing key counts as not equal"},

		// --- Leaf IN ---
		{&FilterNode{Cmp: FilterCompareIn, Key: "region", Vals: []string{"us", "eu"}}, map[string]string{"region": "us"}, true, "IN value present"},
		{&FilterNode{Cmp: FilterCompareIn, Key: "region", Vals: []string{"us", "eu"}}, map[string]string{"region": "asia"}, false, "IN value absent"},
		{&FilterNode{Cmp: FilterCompareIn, Key: "region", Vals: []string{"us", "eu"}}, map[string]string{}, false, "IN missing key"},

		// --- Leaf NOT_IN ---
		{&FilterNode{Cmp: FilterCompareNotIn, Key: "region", Vals: []string{"us", "eu"}}, map[string]string{"region": "asia"}, true, "NOT_IN value absent"},
		{&FilterNode{Cmp: FilterCompareNotIn, Key: "region", Vals: []string{"us", "eu"}}, map[string]string{"region": "us"}, false, "NOT_IN value present"},
		{&FilterNode{Cmp: FilterCompareNotIn, Key: "region", Vals: []string{"us", "eu"}}, map[string]string{}, true, "NOT_IN missing key counts as not in set"},

		// --- Leaf EXISTS ---
		{&FilterNode{Cmp: FilterCompareExists, Key: "debug"}, map[string]string{"debug": "1"}, true, "EXISTS present"},
		{&FilterNode{Cmp: FilterCompareExists, Key: "debug"}, map[string]string{}, false, "EXISTS missing"},

		// --- Leaf NOT_EXISTS ---
		{&FilterNode{Cmp: FilterCompareNotExists, Key: "debug"}, map[string]string{}, true, "NOT_EXISTS missing"},
		{&FilterNode{Cmp: FilterCompareNotExists, Key: "debug"}, map[string]string{"debug": "1"}, false, "NOT_EXISTS present"},

		// -- Numeric comparisons ---
		{&FilterNode{Cmp: FilterCompareGT, Key: "amount", Val: "42"}, map[string]string{"amount": "100"}, true, "GT greater"},
		{&FilterNode{Cmp: FilterCompareGT, Key: "amount", Val: "42"}, map[string]string{"amount": "10"}, false, "GT less"},
		{&FilterNode{Cmp: FilterCompareGTE, Key: "amount", Val: "42"}, map[string]string{"amount": "100"}, true, "GTE greater"},
		{&FilterNode{Cmp: FilterCompareGTE, Key: "amount", Val: "42"}, map[string]string{"amount": "42"}, true, "GTE equal"},
		{&FilterNode{Cmp: FilterCompareGTE, Key: "amount", Val: "42"}, map[string]string{"amount": "42.00000001"}, true, "GTE equal close"},
		{&FilterNode{Cmp: FilterCompareGTE, Key: "amount", Val: "42"}, map[string]string{"amount": "41.99999999"}, false, "GTE equal close"},
		{&FilterNode{Cmp: FilterCompareGTE, Key: "amount", Val: "42"}, map[string]string{"amount": "10"}, false, "GTE less"},
		{&FilterNode{Cmp: FilterCompareLT, Key: "amount", Val: "42"}, map[string]string{"amount": "10"}, true, "LT less"},
		{&FilterNode{Cmp: FilterCompareLT, Key: "amount", Val: "42"}, map[string]string{"amount": "100"}, false, "LT greater"},
		{&FilterNode{Cmp: FilterCompareLT, Key: "amount", Val: "42"}, map[string]string{"amount": "42"}, false, "LT equal"},
		{&FilterNode{Cmp: FilterCompareLTE, Key: "amount", Val: "42"}, map[string]string{"amount": "10"}, true, "LTE less"},
		{&FilterNode{Cmp: FilterCompareLTE, Key: "amount", Val: "42"}, map[string]string{"amount": "42"}, true, "LTE equal"},
		{&FilterNode{Cmp: FilterCompareLTE, Key: "amount", Val: "42"}, map[string]string{"amount": "100"}, false, "LTE greater"},

		// --- AND ---
		{
			&FilterNode{
				Op: FilterOpAnd,
				Nodes: []*FilterNode{
					{Cmp: FilterCompareEQ, Key: "env", Val: "prod"},
					{Cmp: FilterCompareExists, Key: "version"},
				},
			},
			map[string]string{"env": "prod", "version": "1.0"},
			true,
			"AND both children true",
		},
		{
			&FilterNode{
				Op: FilterOpAnd,
				Nodes: []*FilterNode{
					{Cmp: FilterCompareEQ, Key: "env", Val: "prod"},
					{Cmp: FilterCompareExists, Key: "version"},
				},
			},
			map[string]string{"env": "prod"},
			false,
			"AND one child false",
		},

		// --- OR ---
		{
			&FilterNode{
				Op: FilterOpOr,
				Nodes: []*FilterNode{
					{Cmp: FilterCompareEQ, Key: "env", Val: "prod"},
					{Cmp: FilterCompareEQ, Key: "env", Val: "staging"},
				},
			},
			map[string]string{"env": "staging"},
			true,
			"OR one child true",
		},
		{
			&FilterNode{
				Op: FilterOpOr,
				Nodes: []*FilterNode{
					{Cmp: FilterCompareEQ, Key: "env", Val: "prod"},
					{Cmp: FilterCompareEQ, Key: "env", Val: "staging"},
				},
			},
			map[string]string{"env": "qa"},
			false,
			"OR both children false",
		},

		// --- NOT ---
		{
			&FilterNode{
				Op: FilterOpNot,
				Nodes: []*FilterNode{
					{Cmp: FilterCompareExists, Key: "debug"},
				},
			},
			map[string]string{},
			true,
			"NOT EXISTS key missing",
		},
		{
			&FilterNode{
				Op: FilterOpNot,
				Nodes: []*FilterNode{
					{Cmp: FilterCompareExists, Key: "debug"},
				},
			},
			map[string]string{"debug": "1"},
			false,
			"NOT EXISTS key present",
		},

		// --- Nested complex filter ---
		{
			&FilterNode{
				Op: FilterOpOr,
				Nodes: []*FilterNode{
					{
						Op: FilterOpAnd,
						Nodes: []*FilterNode{
							{Cmp: FilterCompareEQ, Key: "env", Val: "prod"},
							{Cmp: FilterCompareIn, Key: "region", Vals: []string{"us", "eu"}},
						},
					},
					{
						Op: FilterOpAnd,
						Nodes: []*FilterNode{
							{Cmp: FilterCompareNotEQ, Key: "tier", Val: "bronze"},
							{
								Op: FilterOpNot,
								Nodes: []*FilterNode{
									{Cmp: FilterCompareExists, Key: "debug"},
								},
							},
						},
					},
				},
			},
			map[string]string{"env": "staging", "region": "us"},
			true,
			"nested complex OR filter",
		},
	}

	for i, tt := range tests {
		d, err := json.Marshal(tt.filter)
		require.NoError(t, err)
		err = FilterValidate(tt.filter)
		require.NoError(t, err)
		t.Logf("test %d: %s over %#v, expected: %v", i, d, tt.tags, tt.expected)
		got, err := FilterMatch(tt.filter, tt.tags)
		if err != nil {
			t.Errorf("case %d (%s): unexpected error: %v", i, tt.desc, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("case %d (%s): expected %v, got %v", i, tt.desc, tt.expected, got)
		}
	}
}

func TestInvalidFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter *FilterNode
	}{
		{
			name: "leaf missing cmp",
			filter: &FilterNode{
				Op:   FilterOpLeaf,
				Key:  "foo",
				Val:  "bar",
				Cmp:  "",
				Vals: nil,
			},
		},
		{
			name: "leaf eq with vals set",
			filter: &FilterNode{
				Op:   FilterOpLeaf,
				Key:  "foo",
				Cmp:  FilterCompareEQ,
				Val:  "bar",
				Vals: []string{"baz"},
			},
		},
		{
			name: "in without vals",
			filter: &FilterNode{
				Op:  FilterOpLeaf,
				Key: "foo",
				Cmp: FilterCompareIn,
			},
		},
		{
			name: "in with val instead of vals",
			filter: &FilterNode{
				Op:  FilterOpLeaf,
				Key: "foo",
				Cmp: FilterCompareIn,
				Val: "bar",
			},
		},
		{
			name: "exists with val set",
			filter: &FilterNode{
				Op:  FilterOpLeaf,
				Key: "foo",
				Cmp: FilterCompareExists,
				Val: "bar",
			},
		},
		{
			name: "nex with vals set",
			filter: &FilterNode{
				Op:   FilterOpLeaf,
				Key:  "foo",
				Cmp:  FilterCompareNotExists,
				Vals: []string{"a"},
			},
		},
		{
			name: "and without children",
			filter: &FilterNode{
				Op: FilterOpAnd,
			},
		},
		{
			name: "or without children",
			filter: &FilterNode{
				Op: FilterOpOr,
			},
		},
		{
			name: "not with no children",
			filter: &FilterNode{
				Op: FilterOpNot,
			},
		},
		{
			name: "not with more than one child",
			filter: &FilterNode{
				Op: FilterOpNot,
				Nodes: []*FilterNode{
					{Op: FilterOpLeaf, Key: "foo", Cmp: FilterCompareEQ, Val: "bar"},
					{Op: FilterOpLeaf, Key: "baz", Cmp: FilterCompareEQ, Val: "qux"},
				},
			},
		},
		{
			name: "leaf without key",
			filter: &FilterNode{
				Op:  FilterOpLeaf,
				Cmp: FilterCompareEQ,
				Val: "bar",
			},
		},
		{
			name: "invalid op",
			filter: &FilterNode{
				Op: "xxx",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FilterValidate(tt.filter)
			require.Error(t, err, "expected error but got nil")
		})
	}

	// A valid case (sanity check)
	valid := &FilterNode{
		Op:  FilterOpLeaf,
		Key: "foo",
		Cmp: FilterCompareEQ,
		Val: "bar",
	}
	require.NoError(t, FilterValidate(valid))
}

func buildComplexFilter() *FilterNode {
	return &FilterNode{
		Op: FilterOpOr,
		Nodes: []*FilterNode{
			{
				Op: FilterOpAnd,
				Nodes: []*FilterNode{
					{Key: "env", Cmp: FilterCompareEQ, Val: "prod"},
					{Key: "region", Cmp: FilterCompareIn, Vals: []string{"us", "eu"}},
				},
			},
			{
				Op: FilterOpAnd,
				Nodes: []*FilterNode{
					{Key: "tier", Cmp: FilterCompareNotEQ, Val: "bronze"},
					{
						Op: FilterOpNot,
						Nodes: []*FilterNode{
							{Key: "debug", Cmp: FilterCompareExists},
						},
					},
				},
			},
		},
	}
}

func TestFilterMatchesComplex(t *testing.T) {
	filter := buildComplexFilter()

	cases := []struct {
		tags     map[string]string
		expected bool
	}{
		{map[string]string{"env": "prod", "region": "us"}, true},
		{map[string]string{"tier": "silver"}, true},
		{map[string]string{"env": "staging", "region": "us"}, true},
	}

	for i, tt := range cases {
		got, err := FilterMatch(filter, tt.tags)
		if err != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
		}
		if got != tt.expected {
			t.Errorf("case %d: expected %v, got %v", i, tt.expected, got)
		}
	}
}

func BenchmarkFilter10k(b *testing.B) {
	ns := []int{10, 100, 1000, 10000}
	const subscribers = 10000

	pubTags := map[string]string{"env": "prod", "region": "us"}

	for _, n := range ns {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			subs := make([]*FilterNode, subscribers)
			for i := 0; i < subscribers; i++ {
				idx := i % n
				subs[i] = buildComplexFilterVariant(idx)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				for _, sub := range subs {
					_, _ = FilterMatch(sub, pubTags)
				}
			}
		})
	}
}

func buildComplexFilterVariant(i int) *FilterNode {
	// Base filter
	base := &FilterNode{
		Op: FilterOpAnd,
		Nodes: []*FilterNode{
			{Key: "env", Cmp: FilterCompareEQ, Val: "prod"},
			{Key: "region", Cmp: FilterCompareIn, Vals: []string{"us", "eu"}},
		},
	}
	// Add a simple extra Or node to make each variant different
	var extra *FilterNode
	if i > 0 {
		extra = &FilterNode{
			Op: FilterOpOr,
			Nodes: []*FilterNode{
				{Key: fmt.Sprintf("extra-%d", i), Cmp: FilterCompareEQ, Val: "1"},
			},
		}
		base = &FilterNode{
			Op:    FilterOpAnd,
			Nodes: []*FilterNode{base, extra},
		}
	}
	return base
}

func BenchmarkFilter10kCachedN(b *testing.B) {
	ns := []int{10, 100, 1000, 10000}
	const subscribers = 10000

	pubTags := map[string]string{"env": "prod", "region": "us"}

	for _, n := range ns {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			subs := make([]*FilterNode, subscribers)
			subHashes := make([][32]byte, subscribers)
			for i := 0; i < subscribers; i++ {
				idx := i % n
				subs[i] = buildComplexFilterVariant(idx)
				subHashes[i] = FilterHash(subs[i])
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				cache := make(map[[32]byte]bool) // simple per-iteration cache
				for i, sub := range subs {
					key := subHashes[i]
					if res, ok := cache[key]; ok {
						_ = res
						continue
					}
					res, _ := FilterMatch(sub, pubTags)
					cache[key] = res
				}
			}
		})
	}
}

var evaluationCachePool = sync.Pool{
	New: func() interface{} {
		// Pre-size for largest expected case to avoid map growth.
		return make(map[[32]byte]bool, 10000)
	},
}

// Alternative sync.Pool version using clear() for Go 1.21+
func BenchmarkFilter10kCachedN_SyncPoolWithClear(b *testing.B) {
	ns := []int{10, 100, 1000, 10000}
	const subscribers = 10000

	pubTags := map[string]string{"env": "prod", "region": "us"}

	for _, n := range ns {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			subs := make([]*FilterNode, subscribers)
			subHashes := make([][32]byte, subscribers)
			for i := 0; i < subscribers; i++ {
				idx := i % n
				subs[i] = buildComplexFilterVariant(idx)
				subHashes[i] = FilterHash(subs[i])
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				cache := evaluationCachePool.Get().(map[[32]byte]bool)

				for i, sub := range subs {
					key := subHashes[i]
					if res, ok := cache[key]; ok {
						_ = res
						continue
					}
					res, _ := FilterMatch(sub, pubTags)
					cache[key] = res
				}

				// Much faster clearing in Go 1.21+
				clear(cache)
				evaluationCachePool.Put(cache)
			}
		})
	}
}

// Different sized pools based on subscriber counts.
var (
	// Pool for up to 100 possible unique filters.
	smallCachePool = sync.Pool{
		New: func() interface{} {
			return make(map[[32]byte]bool, 100)
		},
	}

	// Pool for ~100-1000 possible unique filters.
	mediumCachePool = sync.Pool{
		New: func() interface{} {
			return make(map[[32]byte]bool, 1000)
		},
	}

	// Pool for ~1000-10000 possible unique filters.
	largeCachePool = sync.Pool{
		New: func() interface{} {
			return make(map[[32]byte]bool, 10000)
		},
	}

	// Pool for ~10000+ possible unique filters.
	xLargeCachePool = sync.Pool{
		New: func() interface{} {
			return make(map[[32]byte]bool, 100000)
		},
	}
)

// Function to select appropriate pool based on subscriber count
func getEvaluationsCachePool(numSubscribers int) *sync.Pool {
	switch {
	case numSubscribers <= 100:
		return &smallCachePool
	case numSubscribers <= 1000:
		return &mediumCachePool
	case numSubscribers <= 10000:
		return &largeCachePool
	default:
		return &xLargeCachePool
	}
}

// Benchmark with dynamic pool selection
func BenchmarkFilter10kCachedN_SyncPoolSized(b *testing.B) {
	ns := []int{10, 100, 1000, 10000}
	const subscribers = 10000

	pubTags := map[string]string{"env": "prod", "region": "us"}

	for _, n := range ns {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			subs := make([]*FilterNode, subscribers)
			subHashes := make([][32]byte, subscribers)

			for i := 0; i < subscribers; i++ {
				idx := i % n
				subs[i] = buildComplexFilterVariant(idx)
				subHashes[i] = FilterHash(subs[i])
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				pool := getEvaluationsCachePool(subscribers)
				cache := pool.Get().(map[[32]byte]bool)

				for i, sub := range subs {
					key := subHashes[i]
					if res, ok := cache[key]; ok {
						_ = res
						continue
					}
					res, _ := FilterMatch(sub, pubTags)
					cache[key] = res
				}

				// Clear and return to appropriate pool.
				clear(cache)
				pool.Put(cache)
			}
		})
	}
}

func BenchmarkFilterNumeric10k(b *testing.B) {
	// Build a filter that requires both an integer and a float condition to pass
	buildNumericFilter := func() *FilterNode {
		return &FilterNode{
			Op: FilterOpAnd,
			Nodes: []*FilterNode{
				{
					Op:  FilterOpLeaf,
					Key: "count",
					Cmp: FilterCompareGT, // integer comparison
					Val: "42",
				},
				{
					Op:  FilterOpLeaf,
					Key: "price",
					Cmp: FilterCompareGTE, // float comparison
					Val: "99.5",
				},
			},
		}
	}

	const subscribers = 10000

	// Example publication tags
	tags := map[string]string{
		"count": "100",
		"price": "120.0",
	}

	// Simulate 10k subscribers using the same filter
	subs := make([]*FilterNode, subscribers)
	for i := 0; i < subscribers; i++ {
		subs[i] = buildNumericFilter()
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for _, sub := range subs {
			_, _ = FilterMatch(sub, tags)
		}
	}
}

func BenchmarkFilterNumeric10kCached(b *testing.B) {
	// Build a filter that requires both an integer and a float condition to pass
	buildNumericFilter := func() *FilterNode {
		return &FilterNode{
			Op: FilterOpAnd,
			Nodes: []*FilterNode{
				{
					Op:  FilterOpLeaf,
					Key: "count",
					Cmp: FilterCompareGT, // integer comparison
					Val: "42",
				},
				{
					Op:  FilterOpLeaf,
					Key: "price",
					Cmp: FilterCompareGTE, // float comparison
					Val: "99.5",
				},
			},
		}
	}

	const subscribers = 10000

	// Example publication tags
	tags := map[string]string{
		"count": "100",
		"price": "120.0",
	}

	hashes := make([][32]byte, subscribers)
	subs := make([]*FilterNode, subscribers)
	for i := 0; i < subscribers; i++ {
		subs[i] = buildNumericFilter() // each sub has separate pointer
		hashes[i] = FilterHash(subs[i])
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		cache := make(map[[32]byte]bool) // simple per-iteration cache.
		for i, sub := range subs {
			key := hashes[i]
			if res, ok := cache[key]; ok {
				_ = res
				continue
			}
			res, _ := FilterMatch(sub, tags)
			cache[key] = res
		}
	}
}

func TestTagsFilter_FilterHashConsistency(t *testing.T) {
	// Test that identical filters produce the same hash
	filter1 := &FilterNode{
		Cmp: FilterCompareEQ,
		Key: "env",
		Val: "prod",
	}

	filter2 := &FilterNode{
		Cmp: FilterCompareEQ,
		Key: "env",
		Val: "prod",
	}

	hash1 := FilterHash(filter1)
	hash2 := FilterHash(filter2)

	require.Equal(t, hash1, hash2, "Identical filters should produce the same hash")

	// Test that different filters produce different hashes
	filter3 := &FilterNode{
		Cmp: FilterCompareEQ,
		Key: "env",
		Val: "staging",
	}

	hash3 := FilterHash(filter3)
	require.NotEqual(t, hash1, hash3, "Different filters should produce different hashes")
}
