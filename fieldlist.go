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

// QField returns a query field based on a FieldListItem
func (fi FieldListItem) QField() *queryField {
	val := ":" + fi.Name
	for _, opt := range []string{"relation", "belongs"} {
		_, ok := fi.Options[opt]
		if ok {
			return nil
		}
	}
	field := &queryField{}
	field.key = fi.Name
	field.opts = fi.Options
	field.val = val
	field.eq = fi.Name + "=" + val
	return field
}
