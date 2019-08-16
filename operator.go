package dmpr

import (
	"fmt"
	"reflect"
)

// Operator describes an operator, in which queries can build their WHERE clauses.
type Operator interface {
	Where() string
	Value() []interface{}
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
func (c *ColumnValue) Value() []interface{} {
	return []interface{}{c.value}
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

func (op *EQ) Where() string {
	if op.Value() == nil {
		return fmt.Sprintf("%s IS NULL", op.Column())
	}
	val := reflect.ValueOf(op.Value())
	tmpl := "%s = :%s"
	if val.Kind() == reflect.Slice {
		if val.Len() != 1 {
			return ""
		}
		val = val.Index(0)
		if val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		switch val.Kind() {
		case reflect.Invalid:
			return fmt.Sprintf("%s IS NULL", op.Column())
		case reflect.Slice:
			len := val.Len()
			if len < 1 {
				return ""
			}
			tmpl = "%s IN (:%s)"
		}
	}
	return fmt.Sprintf(tmpl, op.Column(), op.Column())
}
