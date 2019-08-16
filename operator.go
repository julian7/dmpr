package dmpr

import (
	"fmt"
	"reflect"
	"strings"
)

// Operator describes an operator, in which queries can build their WHERE clauses.
type Operator interface {
	Where(bool) string
	Value() map[string]interface{}
}

// ColumnValue is a standard struct representing a database column and its desierd
// value. This is the base struct of column-based operators.
type ColumnValue struct {
	column string
	value  interface{}
}

// Column returns returns the object's column name
func (c *ColumnValue) Column() string {
	return c.column
}

// Value returns the object's value
func (c *ColumnValue) Value() map[string]interface{} {
	return map[string]interface{}{c.column: c.value}
}

// EQ implements equivalence Operator. It is based on Column struct.
type EQ struct {
	ColumnValue
}

// Eq returns an equivalence operator, requesting a certain column should have
// a certain value. It handles arrays too.
func Eq(col string, value interface{}) *EQ {
	return &EQ{ColumnValue: ColumnValue{column: col, value: value}}
}

// Where returns a where clause for the equation. It handles nil, scalar, and slice values.
func (op *EQ) Where(truthy bool) string {
	item := op.Value()[op.Column()]
	if item == nil {
		return fmt.Sprintf("%s IS%s NULL", op.Column(), map[bool]string{true: "", false: " NOT"}[truthy])
	}
	val := reflect.ValueOf(item)
	switch val.Kind() {
	case reflect.Invalid:
		return fmt.Sprintf("%s IS%s NULL", op.Column(), map[bool]string{true: "", false: " NOT"}[truthy])
	case reflect.Slice:
		len := val.Len()
		if len < 1 {
			return ""
		}
		return fmt.Sprintf(
			"%s %sIN (:%s)",
			op.Column(),
			map[bool]string{true: "", false: "NOT "}[truthy],
			op.Column(),
		)
	}
	return fmt.Sprintf(
		"%s %s :%s",
		op.Column(),
		map[bool]string{true: "=", false: "<>"}[truthy],
		op.Column(),
	)
}

// NOT is a simple negate operator struct
type NOT struct {
	Operator
}

// Not returns a negating version for an already existing operator.
func Not(op Operator) *NOT {
	return &NOT{Operator: op}
}

// Where is calls the original operator with flipping truthy flag
func (op *NOT) Where(truthy bool) string {
	return op.Operator.Where(!truthy)
}

type Grouper interface {
	Add(...Operator)
}

// GroupOperator is an iternal data structure for an operator with multiple sub-operators
type GroupOperator struct {
	items []Operator
	op    string
}

// NewGroupOp returns a new group of operators
func NewGroupOp(op string, items ...Operator) *GroupOperator {
	return &GroupOperator{items: items, op: op}
}

// Add adds more operatorn to an existing GroupOperator
func (op *GroupOperator) Add(ops ...Operator) {
	op.items = append(op.items, ops...)
}

// Value returns all the values found in its sub-operators
func (op *GroupOperator) Value() map[string]interface{} {
	values := map[string]interface{}{}
	for _, item := range op.items {
		for key, val := range item.Value() {
			values[key] = val
		}
	}
	return values
}

// Where is a helper function for implementer structs to provide all where clauses
func (op *GroupOperator) Where(truthy bool) string {
	if len(op.items) == 1 {
		return op.items[0].Where(truthy)
	}
	whereClauses := make([]string, 0, len(op.items))
	for _, item := range op.items {
		where := item.Where(true)
		if _, ok := item.(Grouper); ok {
			where = fmt.Sprintf("(%s)", item.Where(true))
		}
		whereClauses = append(whereClauses, where)
	}
	ret := strings.Join(whereClauses, fmt.Sprintf(" %s ", op.op))
	if !truthy {
		ret = fmt.Sprintf("NOT (%s)", ret)
	}
	return ret
}

// AND is an operator struct, with AND relation between each item
type AND struct {
	*GroupOperator
}

// And returns a new AND operator with initial values
func And(ops ...Operator) *AND {
	op := &AND{GroupOperator: NewGroupOp("AND", []Operator{}...)}
	op.Add(ops...)
	return op
}

// Add adds more items into the AND operator
func (op *AND) Add(ops ...Operator) {
	for _, item := range ops {
		if andop, ok := item.(*AND); ok {
			op.GroupOperator.Add(andop.items...)
		} else {
			op.GroupOperator.Add(item)
		}
	}
}

// And is an alias to Add, and it returns itself for chaining
func (op *AND) And(ops ...Operator) *AND {
	op.Add(ops...)
	return op
}

// Where returns the where clause of all sub-operators stringed together into an AND clause
func (op *AND) Where(truthy bool) string {
	return op.GroupOperator.Where(truthy)
}

// OR is an operator struct, with OR relation between each item
type OR struct {
	*GroupOperator
}

// And returns a new OR operator with initial values
func Or(ops ...Operator) *OR {
	op := &OR{GroupOperator: NewGroupOp("OR", []Operator{}...)}
	op.Add(ops...)
	return op
}

// Add adds more items into the OR operator
func (op *OR) Add(ops ...Operator) {
	for _, item := range ops {
		if andop, ok := item.(*OR); ok {
			op.GroupOperator.Add(andop.items...)
		} else {
			op.GroupOperator.Add(item)
		}
	}
}

// Or is an alias to Add, and it returns itself for chaining
func (op *OR) Or(ops ...Operator) *OR {
	op.Add(ops...)
	return op
}

// Where returns the where clause of all sub-operators stringed together into an OR clause
func (op *OR) Where(truthy bool) string {
	return op.GroupOperator.Where(truthy)
}
