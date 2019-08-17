package dmpr

import (
	"reflect"
)

// FieldListItem is a line item of a model's field list
type FieldListItem struct {
	reflect.Type
	Field   *reflect.StructField
	Name    string
	Options map[string]string
}
