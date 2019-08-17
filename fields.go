package dmpr

import (
	"strings"

	"github.com/pkg/errors"
)

type queryType = int
type queryField struct {
	key string
	val string
}

const (
	selectType = 0
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
		if _, ok := fi.Options["_related_to_"]; ok {
			continue
		}
		field := computeField(fi, qt)
		if field != nil {
			queryFields = append(queryFields, *field)
		}
	}
	return queryFields, nil
}

func (m *Mapper) relatedFieldsFor(model interface{}, relation string, qt queryType) ([]queryField, error) {
	if model == nil {
		return nil, errors.New("empty model")
	}
	fields := m.FieldList(model)
	subfields := []FieldListItem{}
	for _, field := range fields {
		if subfield, ok := field.Options["_related_to_"]; ok && relation == subfield {
			subfields = append(subfields, field)
		}
	}
	queryFields := make([]queryField, 0, len(subfields))

	for _, fi := range fields {
		rel := relation + "."
		if !strings.HasPrefix(fi.Name, rel) {
			continue
		}
		fi.Name = strings.TrimPrefix(fi.Name, rel)

		field := computeField(fi, qt)
		if field != nil {
			queryFields = append(queryFields, *field)
		}
	}
	return queryFields, nil

}

func computeField(fi FieldListItem, qt int) *queryField {
	val := ":" + fi.Name
	for _, opt := range []string{"relation", "belongs"} {
		_, ok := fi.Options[opt]
		if ok {
			return nil
		}
	}
	if qt != selectType {
		if _, ok := fi.Options["omitempty"]; ok && isEmptyValue(fi.Value) {
			if (fi.Name == "created_at" && qt == insertType) ||
				(fi.Name == "updated_at" && qt == updateType) {
				val = "NOW()"
			} else {
				return nil
			}
		}
	}
	field := &queryField{}
	field.key = fi.Name
	switch qt {
	case selectType, insertType:
		field.val = val
	case updateType:
		field.val = fi.Name + "=" + val
	}
	return field
}
