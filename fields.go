package dmpr

import (
	"github.com/pkg/errors"
)

type queryType = int
type queryField struct {
	key string
	val string
}

const (
	insertType = 1
	updateType = 2
)

func (m *Mapper) fieldsFor(model interface{}, qt queryType) ([]queryField, error) {
	if model == nil {
		return nil, errors.New("empty model")
	}
	fields := m.FieldList(model)
	queryFields := make([]queryField, 0, len(fields))

	for _, fi := range fields {
		val := ":" + fi.Name
		forcedField := false
		if _, ok := fi.Options["omitempty"]; ok {
			if isEmptyValue(fi.Value) {
				if (fi.Name == "created_at" && qt == insertType) ||
					(fi.Name == "updated_at" && qt == updateType) {
					val = "NOW()"
					forcedField = true
				}
				if !forcedField {
					continue
				}
			}
		}
		field := queryField{}
		field.key = fi.Name
		if qt == insertType {
			field.val = val
		} else if qt == updateType {
			field.val = fi.Name + "=" + val
		}
		queryFields = append(queryFields, field)
	}
	return queryFields, nil
}
