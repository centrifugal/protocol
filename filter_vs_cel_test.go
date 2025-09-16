package protocol

import (
	"testing"

	"github.com/google/cel-go/cel"
)

// Build a zero-alloc FilterNode tree
func buildZeroAllocFilter() *FilterNode {
	return &FilterNode{
		Op: FilterOpAnd,
		Nodes: []*FilterNode{
			{
				Op:  FilterOpLeaf,
				Key: "count",
				Cmp: FilterCompareGT,
				Val: "42",
			},
			{
				Op:  FilterOpLeaf,
				Key: "price",
				Cmp: FilterCompareGTE,
				Val: "99.5",
			},
			{
				Op:  FilterOpLeaf,
				Key: "ticker",
				Cmp: FilterCompareContains,
				Val: "GOO",
			},
		},
	}
}

// Benchmark zero-alloc FilterNode
func BenchmarkFilterCompareFilterNode(b *testing.B) {
	filter := buildZeroAllocFilter()
	const subscribers = 10000

	tags := map[string]string{
		"count":  "100",
		"price":  "120.0",
		"ticker": "GOOG",
	}

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

func BenchmarkFilterCompareCEL(b *testing.B) {
	// CEL environment
	env, err := cel.NewEnv(
		cel.Variable("tags", cel.MapType(cel.StringType, cel.StringType)),
	)
	if err != nil {
		b.Fatal(err)
	}

	expr := `int(tags["count"]) > 42 && double(tags["price"]) >= 99.5 && tags["ticker"].contains("GOO")`

	ast, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		b.Fatal(issues.Err())
	}

	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		b.Fatal(issues.Err())
	}

	prg, err := env.Program(checked)
	if err != nil {
		b.Fatal(err)
	}

	const subscribers = 10000

	subs := make([]cel.Program, subscribers)
	for i := 0; i < subscribers; i++ {
		subs[i] = prg
	}

	tags := map[string]string{
		"count":  "100",
		"price":  "120.0",
		"ticker": "GOOG",
	}

	// Create a shared activation map outside the loop to reduce allocations.
	activation := map[string]any{"tags": tags}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, sub := range subs {
			out, _, err := sub.Eval(activation)
			if err != nil {
				b.Fatal(err)
			}
			// cast to bool
			if out.Value().(bool) != true {
				b.Fatal("unexpected result")
			}
		}
	}
}

func BenchmarkFilterCompareMemoryFilterNode(b *testing.B) {
	for b.Loop() {
		for range 10000 {
			buildZeroAllocFilter()
		}
	}
}

// Benchmark memory usage / allocations for CEL
func BenchmarkFilterCompareCELMemory(b *testing.B) {
	for b.Loop() {
		for range 10000 {
			env, err := cel.NewEnv(
				cel.Variable("tags", cel.MapType(cel.StringType, cel.StringType)),
			)
			if err != nil {
				b.Fatal(err)
			}

			expr := `int(tags["count"]) > 42 && double(tags["price"]) >= 99.5 && tags["ticker"].contains("GOO")`

			ast, issues := env.Parse(expr)
			if issues != nil && issues.Err() != nil {
				b.Fatal(issues.Err())
			}

			checked, issues := env.Check(ast)
			if issues != nil && issues.Err() != nil {
				b.Fatal(issues.Err())
			}

			_, err = env.Program(checked)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
