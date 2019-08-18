package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx/reflectx"
	"github.com/pkg/errors"
)

const (
	RelNotFound = iota
	RelBelongsTo
	RelHasN
	OptRelatedTo = "_related_to_"
	OptBelongs   = "belongs"
	OptRelation  = "relation"
)

type queryField struct {
	key  string
	val  string
	eq   string
	opts map[string]string
}

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

func (mapper *Mapper) FieldMap(model interface{}) map[string]reflect.Value {
	return mapper.Conn.Mapper.FieldMap(indirect(reflect.ValueOf(model)))
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
		if _, ok := fi.Options[OptBelongs]; ok {
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

func FieldsFor(fields []FieldListItem) ([]queryField, error) {
	queryFields := make([]queryField, 0, len(fields))

	for _, fi := range fields {
		if _, ok := fi.Options[OptRelatedTo]; ok {
			continue
		}
		field := fi.QField()
		if field != nil {
			queryFields = append(queryFields, *field)
		}
	}
	return queryFields, nil
}

func RelatedFieldsFor(fields []FieldListItem, relation, tableref string, cb func(reflect.Type) *StructMap) (string, []string, error) {
	for _, field := range fields {
		if field.Name == relation {
			if subfield, ok := field.Options[OptRelation]; ok {
				return HasNFieldsFor(relation, tableref, subfield, field.Type, cb)
			}
			tablename, err := tableNameByType(field.Type)
			if err != nil {
				return "", nil, err
			}
			return BelongsToFieldsFor(fields, relation, tableref, tablename)
		}
	}
	return "", nil, errors.Errorf("Relation %s not found", relation)
}

func BelongsToFieldsFor(fields []FieldListItem, relation, tableref, tablename string) (string, []string, error) {
	joined := fmt.Sprintf("%s %s ON (t1.%s_id=%s.id)", tablename, tableref, relation, tableref)
	selected := []string{}
	rel := len(relation) + 1
FieldScan:
	for _, fi := range fields {
		for _, option := range []string{OptRelation, OptBelongs} {
			if _, ok := fi.Options[option]; ok {
				continue FieldScan
			}
		}
		if subfield, ok := fi.Options[OptRelatedTo]; ok && relation == subfield {
			name := fi.Name[rel:]
			selected = append(selected, fmt.Sprintf("%s.%s AS %s_%s", tableref, name, relation, name))
		}
	}
	return joined, selected, nil
}

func HasNFieldsFor(relation, tableref, relindex string, t reflect.Type, typeMapper func(reflect.Type) *StructMap) (string, []string, error) {
	t = deref(t)
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	tablename, err := tableNameByType(t)
	if err != nil {
		return "", nil, err
	}
	joined := fmt.Sprintf("%s %s ON (t1.id=%s.%s_id)", tablename, tableref, tableref, relindex)
	fields, err := FieldsFor(FieldList(typeMapper(t)))
	if err != nil {
		return "", nil, err
	}

	selected := make([]string, 0, len(fields))
	for _, field := range fields {
		selected = append(selected, fmt.Sprintf("%s.%s AS %s_%s", tableref, field.key, relation, field.key))
	}
	return joined, selected, nil
}

type traversal struct {
	name     string
	index    []int
	relation reflect.StructField
}

func TraversalsByName(tm *StructMap, columns []string) []traversal {
	fields := make([]traversal, 0, len(columns))
	for _, name := range columns {
		trav := traversal{name: name, index: []int{}}
		if !trav.set(tm, name) {
			underscored := strings.Replace(name, "_", ".", 1)
			if !trav.set(tm, underscored) {
				continue
			}
		}
		fields = append(fields, trav)
	}
	return fields
}

func (trav *traversal) set(tm *StructMap, name string) bool {
	fi, ok := tm.Names[name]
	if !ok {
		return false
	}
	trav.index = fi.Index
	if relation, ok := fi.Options[OptRelatedTo]; ok {
		if oi, ok := tm.Names[relation]; ok {
			trav.relation = oi.Field
		}
	}
	delete(tm.Names, name)
	return true
}

// TMP:rewrite
func fieldsByTraversal(v reflect.Value, traversals []traversal, values []interface{}, ptrs bool) error {
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return errors.New("argument not a struct")
	}

	for i, traversal := range traversals {
		if len(traversal.index) == 0 {
			values[i] = new(interface{})
			continue
		}
		f := fieldByIndexes(v, traversal.index)
		if ptrs {
			values[i] = f.Addr().Interface()
		} else {
			values[i] = f.Interface()
		}
	}
	return nil
}
