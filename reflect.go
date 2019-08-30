package dmpr

import (
	"database/sql/driver"
	"reflect"

	"github.com/gobuffalo/flect"
	"github.com/pkg/errors"
)

// ErrInvalidType is an error returning where the model is invalid (currently, when it is a null pointer)
var ErrInvalidType = errors.New("Invalid Model Type")

// Reflect dissects provided model reference into a type and value for further inspection.
// It accepts pointer, slice, and pointer of slices indirections.
func Reflect(model interface{}) (reflect.Type, reflect.Value) {
	value := indirect(reflect.ValueOf(model))
	t := value.Type()
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	return t, value
}

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

// fieldByIndexes dials in to a value by index #s, returning the value inside. It allocates pointers and maps when needed.
func fieldByIndexes(v reflect.Value, indexes []int) reflect.Value {
	for _, i := range indexes {
		v = reflect.Indirect(v)
		if v.Kind() == reflect.Slice {
			v = dialSubindex(v)
		}
		v = indirect(v)
		v = v.Field(i)
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

func dialSubindex(v reflect.Value) reflect.Value {
	if v.Len() > 0 {
		v = indirect(v.Index(0))
	} else {
		t := v.Type().Elem()
		if t.Kind() == reflect.Ptr {
			newVal := reflect.New(deref(t))
			v.Set(reflect.Append(v, newVal))
			v = reflect.Indirect(newVal)
		} else {
			newVal := reflect.New(t)
			v.Set(reflect.Append(v, newVal.Elem()))
			v = newVal
		}
	}
	return v
}

// mergeFields merges a struct's field's slice values into another one
func mergeFields(dst, src reflect.Value) (reflect.Value, error) {
	dst = reflect.Indirect(dst)
	src = reflect.Indirect(src)
	if dst.Kind() != reflect.Struct {
		return dst, errors.New("dst is not a struct")
	}
	if src.Type() != dst.Type() {
		return dst, errors.New("src is not of the same type")
	}
	for idx := 0; idx < dst.NumField(); idx++ {
		dstField := dst.Field(idx)
		srcField := src.Field(idx)
		if dstField.Kind() == reflect.Slice && dstField.Type().Elem() == srcField.Type().Elem() {
			dstField.Set(reflect.AppendSlice(dstField, srcField))
		}
	}

	return dst, nil
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
