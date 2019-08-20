package dmpr

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
)

const (
	// OptRelatedTo is a struct tag option mapper inserts for all columns, which is a subfield of an embedded table. Its value is the name of the embedded table.
	OptRelatedTo = "_related_to_"
	// OptBelongs is a struct tag option marking a "belongs-to" relation.
	OptBelongs = "belongs"
	// OptRelation is a struct tag option marking a "has-one" or "has-many" relation.
	OptRelation = "relation"
)

type queryField struct {
	key  string
	val  string
	eq   string
	opts map[string]string
}

// TypeOf returns the type of a model. It handles pointer, slice, and pointer of slice indirections.
func TypeOf(model interface{}) reflect.Type {
	t := indirect(reflect.ValueOf(model)).Type()
	if t.Kind() == reflect.Slice {
		return t.Elem()
	}
	return t
}

// FieldMap returns a map of fields for a model. It handles pointer of model.
func (m *Mapper) FieldMap(model interface{}) map[string]reflect.Value {
	if err := m.tryOpen(); err != nil {
		log.Warnf("cannot get field map of %+v: %v", model, err)
		return map[string]reflect.Value{}
	}
	return m.Conn.Mapper.FieldMap(indirect(reflect.ValueOf(model)))
}

// FieldsFor converts FieldListItems to query fields SQL query builders can use. It doesn't include related fields.
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

// RelatedFieldsFor converts FieldListItems to JOINs and SELECTs SQL query builders can use directly
func RelatedFieldsFor(fields []FieldListItem, relation, tableref string, cb func(reflect.Type) []FieldListItem) (joins string, selects []string, err error) {
	for _, field := range fields {
		if field.Path == relation {
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
	return "", nil, errors.Errorf("Relation %q not found", relation)
}

// BelongsToFieldsFor converts FieldListItems to JOIN and SELECTs query substrings SQL query buildders can use directly
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
			name := fi.Path[rel:]
			selected = append(selected, fmt.Sprintf("%s.%s AS %s_%s", tableref, name, relation, name))
		}
	}
	return joined, selected, nil
}

// HasNFieldsFor queries related model to build JOIN and SELECTs query substrings SQL query buildders can use directly.
// It uses a callback, which can provide a []FieldListItem from the referenced type.
func HasNFieldsFor(relation, tableref, relindex string, t reflect.Type, typeMapper func(reflect.Type) []FieldListItem) (string, []string, error) {
	t = deref(t)
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	tablename, err := tableNameByType(t)
	if err != nil {
		return "", nil, err
	}
	joined := fmt.Sprintf("%s %s ON (t1.id=%s.%s_id)", tablename, tableref, tableref, relindex)
	fields, err := FieldsFor(typeMapper(t))
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

// fieldByIndexes dials in to a value by index #s, returning the value inside. It allocates pointers and maps when needed.
func fieldByIndexes(v reflect.Value, indexes []int) reflect.Value {
	for _, i := range indexes {
		v = reflect.Indirect(v).Field(i)
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

// fieldsByTraversal TMP:rewrite: fills traversal entries into a slice of models (values) based on traversal indexes
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
