package protocol

import (
	"github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
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

func BenchmarkFilterZeroAlloc(b *testing.B) {
	filter := buildComplexFilter()
	const subscribers = 10000
	pubTags := map[string]string{"env": "prod", "region": "us"}

	subs := make([]*FilterNode, subscribers)
	for i := 0; i < subscribers; i++ {
		subs[i] = filter
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, sub := range subs {
			_, _ = FilterMatch(sub, pubTags)
		}
	}
}

func BenchmarkFilterNumericZeroAlloc(b *testing.B) {
	// Build a filter that requires both an integer and a float condition to pass
	filter := &FilterNode{
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

	const subscribers = 10000

	// Example publication tags
	tags := map[string]string{
		"count": "100",
		"price": "120.0",
	}

	// Simulate 10k subscribers using the same filter
	subs := make([]*FilterNode, subscribers)
	for i := 0; i < subscribers; i++ {
		subs[i] = filter
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, sub := range subs {
			_, _ = FilterMatch(sub, tags)
		}
	}
}
