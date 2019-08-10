package dmpr

import (
	"database/sql/driver"
	"reflect"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/pkg/errors"
)

// ErrInvalidType is an error returning where the model is invalid (currently, when it is a null pointer)
var ErrInvalidType = errors.New("Invalid Model Type")

func coreTypeOf(model interface{}) reflect.Type {
	t := reflect.TypeOf(model)
	for {
		if t == nil {
			return nil
		}
		switch t.Kind() {
		case reflect.Slice, reflect.Array, reflect.Ptr:
			t = t.Elem()
		default:
			return t
		}
	}
}

func tableName(model interface{}) (string, error) {
	t := coreTypeOf(model)
	if t == nil {
		return "", ErrInvalidType
	}
	modelName := strings.ToLower(t.Name())
	if modelName == "" {
		return "", ErrInvalidType
	}
	return flect.Pluralize(modelName), nil
}

func (mapper *Mapper) FieldList(model interface{}) []FieldListItem {
	v := reflect.Indirect(reflect.ValueOf(model))
	tm := mapper.Conn.Mapper.TypeMap(v.Type())
	fields := make([]FieldListItem, 0, len(tm.Names))
	for _, fi := range tm.Index {
		if strings.Index(fi.Path, ".") > 0 || len(fi.Index) < 1 {
			continue
		}
		fields = append(fields, FieldListItem{
			Name:    fi.Path,
			Options: fi.Options,
			Value:   v.Field(fi.Index[0]),
		})
	}
	return fields
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
