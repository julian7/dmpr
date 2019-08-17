package dmpr

import (
	"reflect"
	"strings"

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

func (mapper *Mapper) FieldList(model interface{}) []FieldListItem {
	related := []string{}
	t := typeOf(indirect(reflect.ValueOf(model)))
	tm := mapper.Conn.Mapper.TypeMap(t)
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
		fieldStruct := t.FieldByIndex(fi.Index)
		fields = append(fields, FieldListItem{
			Name:    fi.Path,
			Options: fi.Options,
			Field:   &fieldStruct,
			Type:    fieldStruct.Type,
		})
	}
	return fields
}

func typeOf(v reflect.Value) reflect.Type {
	t := v.Type()
	if t.Kind() == reflect.Slice {
		return t.Elem()
	}
	return t
}
