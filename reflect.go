package dmpr

import (
	"reflect"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/pkg/errors"
)

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

func (m *Mapper) fieldMap(model interface{}) map[string]reflect.Value {
	v := reflect.Indirect(reflect.ValueOf(model))
	return m.Conn.Mapper.FieldMap(v)
}
