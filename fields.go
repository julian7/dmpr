package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx/reflectx"
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

// StructMap is an extension of reflectx.StructMap (from sqlx), to store the structure's type too.
type StructMap struct {
	*reflectx.StructMap
	Type reflect.Type
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

// TypeMap returns a map of types in the form of a StructMap, from the original model's type
func (m *Mapper) TypeMap(t reflect.Type) *StructMap {
	if err := m.tryOpen(); err != nil {
		log.Warnf("cannot get type map of %+v: %v", t, err)
		return nil
	}
	return &StructMap{Type: t, StructMap: m.Conn.Mapper.TypeMap(t)}
}

// FieldList reads StructMap, and returns a slice of FieldListItem. It detects subfields of belonging items, while flagging subfields with OptRelatedTo option in the StructMap, for future use (see RelatedFieldsFor and BelongsToFieldsFor)
func FieldList(tm *StructMap) []FieldListItem {
	related := []string{}

	for _, fi := range tm.Index {
		if _, ok := fi.Options[OptBelongs]; ok {
			related = append(related, fi.Path)
		}
	}
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
func RelatedFieldsFor(fields []FieldListItem, relation, tableref string, cb func(reflect.Type) *StructMap) (joins string, selects []string, err error) {
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
			name := fi.Name[rel:]
			selected = append(selected, fmt.Sprintf("%s.%s AS %s_%s", tableref, name, relation, name))
		}
	}
	return joined, selected, nil
}

// HasNFieldsFor queries related model to build JOIN and SELECTs query substrings SQL query buildders can use directly.
// It uses a callback, which can provide a StructMap from the referenced type.
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

// TraversalsByName provides a traversal index for SELECT query results, to map result rows' columns with model's entry positions
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

// set pulls Index information from the StructMap by column name, marking the name as read to prevent idempotency
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
