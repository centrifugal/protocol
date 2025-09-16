package protocol

func Eq(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareEQ, Val: val}
}

func Neq(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareNotEQ, Val: val}
}

func In(key string, vals ...string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareIn, Vals: vals}
}

func Nin(key string, vals ...string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareNotIn, Vals: vals}
}

func Gt(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareGT, Val: val}
}

func Gte(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareGTE, Val: val}
}

func Lt(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareLT, Val: val}
}

func Lte(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareLTE, Val: val}
}

func Contains(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareContains, Val: val}
}

func Starts(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterComparePrefix, Val: val}
}

func Ends(key, val string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareSuffix, Val: val}
}

func Exists(key string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareExists}
}

func NotExists(key string) *FilterNode {
	return &FilterNode{Op: "", Key: key, Cmp: FilterCompareNotExists}
}

// And combines multiple FilterNode children with logical AND
func And(nodes ...*FilterNode) *FilterNode {
	return &FilterNode{Op: FilterOpAnd, Nodes: nodes}
}

// Or combines multiple FilterNode children with logical OR
func Or(nodes ...*FilterNode) *FilterNode {
	return &FilterNode{Op: FilterOpOr, Nodes: nodes}
}

// Not negates a single FilterNode
func Not(node *FilterNode) *FilterNode {
	return &FilterNode{Op: FilterOpNot, Nodes: []*FilterNode{node}}
}
