package dmpr

import (
	"strings"

	"github.com/pkg/errors"
)

const OptRelatedTo = "_related_to_"

type queryType = int
type queryField struct {
	key  string
	val  string
	opts map[string]string
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
		if _, ok := fi.Options[OptRelatedTo]; ok {
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
		if subfield, ok := field.Options[OptRelatedTo]; ok && relation == subfield {
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
	field := &queryField{}
	field.key = fi.Name
	field.opts = fi.Options
	switch qt {
	case selectType, insertType:
		field.val = val
	case updateType:
		field.val = fi.Name + "=" + val
	}
	return field
}