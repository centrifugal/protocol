package protocol

import "testing"

func TestFilterAll(t *testing.T) {
	tests := []struct {
		filter   *FilterNode
		tags     map[string]string
		expected bool
		desc     string
	}{
		// --- Leaf EQ ---
		{&FilterNode{Compare: FilterCompareEQ, Key: "env", Value: "prod"}, map[string]string{"env": "prod"}, true, "EQ matches"},
		{&FilterNode{Compare: FilterCompareEQ, Key: "env", Value: "prod"}, map[string]string{"env": "staging"}, false, "EQ does not match"},
		{&FilterNode{Compare: FilterCompareEQ, Key: "env", Value: "prod"}, map[string]string{}, false, "EQ missing key"},

		// --- Leaf NOT_EQ ---
		{&FilterNode{Compare: FilterCompareNotEQ, Key: "tier", Value: "bronze"}, map[string]string{"tier": "silver"}, true, "NOT_EQ different value"},
		{&FilterNode{Compare: FilterCompareNotEQ, Key: "tier", Value: "bronze"}, map[string]string{"tier": "bronze"}, false, "NOT_EQ same value"},
		{&FilterNode{Compare: FilterCompareNotEQ, Key: "tier", Value: "bronze"}, map[string]string{}, true, "NOT_EQ missing key counts as not equal"},

		// --- Leaf IN ---
		{&FilterNode{Compare: FilterCompareIn, Key: "region", ValueSet: []string{"us", "eu"}}, map[string]string{"region": "us"}, true, "IN value present"},
		{&FilterNode{Compare: FilterCompareIn, Key: "region", ValueSet: []string{"us", "eu"}}, map[string]string{"region": "asia"}, false, "IN value absent"},
		{&FilterNode{Compare: FilterCompareIn, Key: "region", ValueSet: []string{"us", "eu"}}, map[string]string{}, false, "IN missing key"},

		// --- Leaf NOT_IN ---
		{&FilterNode{Compare: FilterCompareNotIn, Key: "region", ValueSet: []string{"us", "eu"}}, map[string]string{"region": "asia"}, true, "NOT_IN value absent"},
		{&FilterNode{Compare: FilterCompareNotIn, Key: "region", ValueSet: []string{"us", "eu"}}, map[string]string{"region": "us"}, false, "NOT_IN value present"},
		{&FilterNode{Compare: FilterCompareNotIn, Key: "region", ValueSet: []string{"us", "eu"}}, map[string]string{}, true, "NOT_IN missing key counts as not in set"},

		// --- Leaf EXISTS ---
		{&FilterNode{Compare: FilterCompareExists, Key: "debug"}, map[string]string{"debug": "1"}, true, "EXISTS present"},
		{&FilterNode{Compare: FilterCompareExists, Key: "debug"}, map[string]string{}, false, "EXISTS missing"},

		// --- Leaf NOT_EXISTS ---
		{&FilterNode{Compare: FilterCompareNotExists, Key: "debug"}, map[string]string{}, true, "NOT_EXISTS missing"},
		{&FilterNode{Compare: FilterCompareNotExists, Key: "debug"}, map[string]string{"debug": "1"}, false, "NOT_EXISTS present"},

		// --- AND ---
		{
			&FilterNode{
				Op: FilterOpAnd,
				Children: []*FilterNode{
					{Compare: FilterCompareEQ, Key: "env", Value: "prod"},
					{Compare: FilterCompareExists, Key: "version"},
				},
			},
			map[string]string{"env": "prod", "version": "1.0"},
			true,
			"AND both children true",
		},
		{
			&FilterNode{
				Op: FilterOpAnd,
				Children: []*FilterNode{
					{Compare: FilterCompareEQ, Key: "env", Value: "prod"},
					{Compare: FilterCompareExists, Key: "version"},
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
				Children: []*FilterNode{
					{Compare: FilterCompareEQ, Key: "env", Value: "prod"},
					{Compare: FilterCompareEQ, Key: "env", Value: "staging"},
				},
			},
			map[string]string{"env": "staging"},
			true,
			"OR one child true",
		},
		{
			&FilterNode{
				Op: FilterOpOr,
				Children: []*FilterNode{
					{Compare: FilterCompareEQ, Key: "env", Value: "prod"},
					{Compare: FilterCompareEQ, Key: "env", Value: "staging"},
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
				Children: []*FilterNode{
					{Compare: FilterCompareExists, Key: "debug"},
				},
			},
			map[string]string{},
			true,
			"NOT EXISTS key missing",
		},
		{
			&FilterNode{
				Op: FilterOpNot,
				Children: []*FilterNode{
					{Compare: FilterCompareExists, Key: "debug"},
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
				Children: []*FilterNode{
					{
						Op: FilterOpAnd,
						Children: []*FilterNode{
							{Compare: FilterCompareEQ, Key: "env", Value: "prod"},
							{Compare: FilterCompareIn, Key: "region", ValueSet: []string{"us", "eu"}},
						},
					},
					{
						Op: FilterOpAnd,
						Children: []*FilterNode{
							{Compare: FilterCompareNotEQ, Key: "tier", Value: "bronze"},
							{
								Op: FilterOpNot,
								Children: []*FilterNode{
									{Compare: FilterCompareExists, Key: "debug"},
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
		Children: []*FilterNode{
			{
				Op: FilterOpAnd,
				Children: []*FilterNode{
					{Key: "env", Compare: FilterCompareEQ, Value: "prod"},
					{Key: "region", Compare: FilterCompareIn, ValueSet: []string{"us", "eu"}},
				},
			},
			{
				Op: FilterOpAnd,
				Children: []*FilterNode{
					{Key: "tier", Compare: FilterCompareNotEQ, Value: "bronze"},
					{
						Op: FilterOpNot,
						Children: []*FilterNode{
							{Key: "debug", Compare: FilterCompareExists},
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
		Children: []*FilterNode{
			{
				Op:      FilterOpLeaf,
				Key:     "count",
				Compare: FilterCompareIntGT, // integer comparison
				Value:   "42",
			},
			{
				Op:      FilterOpLeaf,
				Key:     "price",
				Compare: FilterCompareFloatGTE, // float comparison
				Value:   "99.5",
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
