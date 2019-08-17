package dmpr

import (
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx/reflectx"
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

type StructMap struct {
	*reflectx.StructMap
	Type reflect.Type
}

func TypeOf(model interface{}) reflect.Type {
	t := indirect(reflect.ValueOf(model)).Type()
	if t.Kind() == reflect.Slice {
		return t.Elem()
	}

	return t
}

func (mapper *Mapper) TypeMap(t reflect.Type) *StructMap {
	return &StructMap{Type: t, StructMap: mapper.Conn.Mapper.TypeMap(t)}
}

func FieldList(tm *StructMap) []FieldListItem {
	related := []string{}
	fields := make([]FieldListItem, 0, len(tm.Names))
	for _, fi := range tm.Index {
		if fi.Parent.Field.Type != nil {
			found := false
			for _, rel := range related {
				if fi.Parent.Path == rel {
					found = true
					fi.Options[OptRelatedTo] = rel
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
		fieldStruct := tm.Type.FieldByIndex(fi.Index)
		fields = append(fields, FieldListItem{
			Name:    fi.Path,
			Options: fi.Options,
			Field:   &fieldStruct,
			Type:    fieldStruct.Type,
		})
	}
	return fields
}

func FieldsFor(fields []FieldListItem, qt queryType) ([]queryField, error) {
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

func RelatedFieldsFor(fields []FieldListItem, relation string, qt queryType) ([]queryField, error) {
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
