package dmpr

import (
	"database/sql/driver"
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
	t = deref(t)
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

// copied from stdlib's encoding/json/encode.go, added driver.Valuer handling
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Struct:
		optional, ok := v.Interface().(driver.Valuer)
		if ok {
			v, _ := optional.Value()
			return v == nil
		}
		return false
	}
	return false
}
