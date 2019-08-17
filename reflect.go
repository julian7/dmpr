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
	return valueName(t)
}

func valueName(t reflect.Type) (string, error) {
	modelName := strings.ToLower(t.Name())
	if modelName == "" {
		return "", ErrInvalidType
	}
	return flect.Pluralize(modelName), nil
}

func (mapper *Mapper) subTableName(model interface{}, fieldName string) (string, error) {
	for _, field := range mapper.FieldList(model) {
		if field.Name == fieldName {
			return valueName(field.Value.Type())
		}
	}
	return "", errors.New("field not found")
}

func (mapper *Mapper) FieldList(model interface{}) []FieldListItem {
	related := []string{}
	v := reflect.Indirect(reflect.ValueOf(model))
	tm := mapper.Conn.Mapper.TypeMap(v.Type())
	fields := make([]FieldListItem, 0, len(tm.Names))
	for _, fi := range tm.Index {
		dot := strings.LastIndex(fi.Path, ".")
		if dot > 0 {
			found := false
			for _, rel := range related {
				if dot == len(rel) && strings.HasPrefix(fi.Path, rel) {
					found = true
					fi.Options["_related_to_"] = rel
					break
				}
			}
			if !found {
				continue
			}
		}
		if _, ok := fi.Options["belongs"]; ok {
			related = append(related, fi.Path)
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
