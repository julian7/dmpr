package dmpr

import (
	"reflect"
	"testing"
)

func TestColumnValue_Value(t *testing.T) {
	want := map[string]interface{}{"one": interface{}("two")}
	c := &ColumnValue{column: "one", value: "two"}
	if got := c.Value(); !reflect.DeepEqual(got, want) {
		t.Errorf("ColumnValue.Value() = %v, want %v", got, want)
	}
}

func TestEQ_Where(t *testing.T) {
	tests := []struct {
		name   string
		col    string
		val    interface{}
		truthy bool
		want   string
	}{
		{
			name:   "nil value",
			col:    "one",
			val:    nil,
			truthy: true,
			want:   "one IS NULL",
		},
		{
			name:   "negative nil value",
			col:    "one",
			truthy: false,
			want:   "one IS NOT NULL",
		},
		{
			name:   "scalar value",
			col:    "one",
			val:    "two",
			truthy: true,
			want:   "one = :one",
		},
		{
			name:   "negative scalar value",
			col:    "one",
			val:    "two",
			truthy: false,
			want:   "one <> :one",
		},
		{
			name:   "slice value",
			col:    "one",
			val:    []string{"two"},
			truthy: true,
			want:   "one IN (:one)",
		},
		{
			name:   "negative slice value",
			col:    "one",
			val:    []string{"two"},
			truthy: false,
			want:   "one NOT IN (:one)",
		},
		{
			name:   "empty slice",
			col:    "one",
			val:    []string{},
			truthy: true,
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := Eq(tt.col, tt.val)
			if got := op.Where(tt.truthy); got != tt.want {
				t.Errorf("EQ.Where() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupOperator_Value(t *testing.T) {
	tests := []struct {
		name  string
		items []Operator
		want  map[string]interface{}
	}{
		{
			name: "zero items",
			want: map[string]interface{}{},
		},
		{
			name:  "one item",
			items: []Operator{Eq("one", "two")},
			want: map[string]interface{}{
				"one": interface{}("two"),
			},
		},
		{
			name:  "multiple items",
			items: []Operator{Eq("one", "two"), Eq("three", "four")},
			want: map[string]interface{}{
				"one":   interface{}("two"),
				"three": interface{}("four"),
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &GroupOperator{
				items: tt.items,
			}
			if got := op.Value(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GroupOperator.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNOT_Where(t *testing.T) {
	tests := []struct {
		name   string
		op     Operator
		truthy bool
		want   string
	}{
		{
			name:   "negating",
			op:     Eq("one", "two"),
			truthy: true,
			want:   "one <> :one",
		},
		{
			name:   "double negating",
			op:     Eq("one", "two"),
			truthy: false,
			want:   "one = :one",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := Not(tt.op)
			if got := op.Where(tt.truthy); got != tt.want {
				t.Errorf("NOT.Where() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnd(t *testing.T) {
	tests := []struct {
		name string
		op   []Operator
		want *AND
	}{
		{
			name: "single item",
			op:   []Operator{Eq("one", "two")},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{Eq("one", "two")}}},
		},
		{
			name: "swallows sub-and item",
			op:   []Operator{And(Eq("one", "two"))},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{Eq("one", "two")}}},
		},
		{
			name: "multiple single items",
			op:   []Operator{And(Eq("one", "two"), Eq("three", "four"))},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
			}}},
		},
		{
			name: "multiple group items",
			op:   []Operator{And(Eq("one", "two"), And(Eq("three", "four"), Eq("five", "six")))},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := And(tt.op...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("And() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupOperator_Where(t *testing.T) {
	tests := []struct {
		name   string
		op     *GroupOperator
		truthy bool
		want   string
	}{
		{
			name:   "single item",
			op:     NewGroupOp("+", Eq("one", "two")),
			truthy: true,
			want:   "one = :one",
		},
		{
			name:   "negated single item",
			op:     NewGroupOp("+", Eq("one", "two")),
			truthy: false,
			want:   "one <> :one",
		},
		{
			name:   "multiple items",
			op:     NewGroupOp("+", Eq("one", "two"), Eq("three", "four"), Eq("five", "six")),
			truthy: true,
			want:   "one = :one + three = :three + five = :five",
		},
		{
			name:   "negate multiple items",
			op:     NewGroupOp("+", Eq("one", "two"), Eq("three", "four"), Eq("five", "six")),
			truthy: false,
			want:   "NOT (one = :one + three = :three + five = :five)",
		},
		{
			name:   "negate multiple items 2",
			op:     NewGroupOp("+", Eq("one", "two"), NewGroupOp("-", Eq("three", "four"), Eq("five", "six"))),
			truthy: false,
			want:   "NOT (one = :one + (three = :three - five = :five))",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.op.Where(tt.truthy); got != tt.want {
				t.Errorf("AND.Where() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAND_And(t *testing.T) {
	tests := []struct {
		name  string
		op    Operator
		other []Operator
		want  *AND
	}{
		{
			name:  "add single item",
			op:    Eq("one", "two"),
			other: []Operator{Eq("three", "four")},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
			}}},
		},
		{
			name:  "add multiple items",
			op:    Eq("one", "two"),
			other: []Operator{Eq("three", "four"), Eq("five", "six")},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
		{
			name:  "add sub-and",
			op:    Eq("one", "two"),
			other: []Operator{And(Eq("three", "four"), Eq("five", "six"))},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
		{
			name:  "multiple .And()s",
			op:    Eq("one", "two"),
			other: []Operator{And(Eq("three", "four")).And(Eq("five", "six"))},
			want: &AND{GroupOperator: &GroupOperator{op: "AND", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := And(tt.op).And(tt.other...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AND.And() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOR_Or(t *testing.T) {
	tests := []struct {
		name  string
		op    Operator
		other []Operator
		want  *OR
	}{
		{
			name:  "add single item",
			op:    Eq("one", "two"),
			other: []Operator{Eq("three", "four")},
			want: &OR{GroupOperator: &GroupOperator{op: "OR", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
			}}},
		},
		{
			name:  "add multiple items",
			op:    Eq("one", "two"),
			other: []Operator{Eq("three", "four"), Eq("five", "six")},
			want: &OR{GroupOperator: &GroupOperator{op: "OR", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
		{
			name:  "add sub-and",
			op:    Eq("one", "two"),
			other: []Operator{Or(Eq("three", "four"), Eq("five", "six"))},
			want: &OR{GroupOperator: &GroupOperator{op: "OR", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
		{
			name:  "multiple .Or()s",
			op:    Eq("one", "two"),
			other: []Operator{Or(Eq("three", "four")).Or(Eq("five", "six"))},
			want: &OR{GroupOperator: &GroupOperator{op: "OR", items: []Operator{
				Eq("one", "two"),
				Eq("three", "four"),
				Eq("five", "six"),
			}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Or(tt.op).Or(tt.other...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OR.Or() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
