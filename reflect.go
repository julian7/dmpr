package dmpr

import (
	"reflect"

	"github.com/gobuffalo/flect"
	"github.com/pkg/errors"
)

// ErrInvalidType is an error returning where the model is invalid (currently, when it is a null pointer)
var ErrInvalidType = errors.New("Invalid Model Type")

func indirect(v reflect.Value) reflect.Value {
	for {
		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			v = v.Elem()
		default:
			return v
		}
	}
}

func deref(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func tableName(model interface{}) (string, error) {
	t := reflect.TypeOf(model)
	for {
		if t == nil {
			break
		}
		k := t.Kind()
		if k == reflect.Slice || k == reflect.Array || k == reflect.Ptr {
			t = t.Elem()
		} else {
			break
		}
	}
	if t == nil {
		return "", ErrInvalidType
	}
	return tableNameByType(t)
}

func tableNameByType(t reflect.Type) (string, error) {
	modelName := flect.Underscore(t.Name())
	if modelName == "" {
		return "", ErrInvalidType
	}
	return flect.Pluralize(modelName), nil
}

func SubTableName(fields []FieldListItem, fieldName string) (string, error) {
	for _, field := range fields {
		if field.Name == fieldName {
			return tableNameByType(field.Type)
		}
	}
	return "", errors.New("field not found")
}

func fieldByIndexes(v reflect.Value, indexes []int) reflect.Value {
	for _, i := range indexes {
		v = reflect.Indirect(v).Field(i)
		if v.Kind() == reflect.Ptr && v.IsNil() {
			alloc := reflect.New(deref(v.Type()))
			v.Set(alloc)
		}
		if v.Kind() == reflect.Map && v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
	}
	return v
}

// func typeOf(v reflect.Value) reflect.Type {
// 	t := v.Type()
// 	if t.Kind() == reflect.Slice {
// 		return t.Elem()
// 	}
// 	return t
// }
