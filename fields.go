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

// FieldList stores fields of a reflectx.StructMap's Index (from sqlx), with the structure's type
type FieldList struct {
	Fields []*reflectx.FieldInfo
	Type   reflect.Type
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
func (m *Mapper) TypeMap(t reflect.Type) *FieldList {
	if err := m.tryOpen(); err != nil {
		log.Warnf("cannot get type map of %+v: %v", t, err)
		return nil
	}
	return &FieldList{Type: t, Fields: m.Conn.Mapper.TypeMap(t).Index}
}

// Itemize reads StructMap, and returns a slice of FieldListItem. It detects subfields of belonging items, while flagging subfields with OptRelatedTo option in the StructMap, for future use (see RelatedFieldsFor and BelongsToFieldsFor)
func (fl *FieldList) Itemize() []FieldListItem {
	related := []string{}
	for _, fi := range fl.Fields {
		if _, ok := fi.Options[OptBelongs]; ok {
			related = append(related, fi.Path)
		}
	}
	fields := make([]FieldListItem, 0, len(fl.Fields))
	for _, fi := range fl.Fields {
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
		fieldStruct := fl.Type.FieldByIndex(fi.Index)
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
func RelatedFieldsFor(fields []FieldListItem, relation, tableref string, cb func(reflect.Type) *FieldList) (joins string, selects []string, err error) {
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
			name := fi.Name[rel:]
			selected = append(selected, fmt.Sprintf("%s.%s AS %s_%s", tableref, name, relation, name))
		}
	}
	return joined, selected, nil
}

// HasNFieldsFor queries related model to build JOIN and SELECTs query substrings SQL query buildders can use directly.
// It uses a callback, which can provide a FieldList from the referenced type.
func HasNFieldsFor(relation, tableref, relindex string, t reflect.Type, typeMapper func(reflect.Type) *FieldList) (string, []string, error) {
	t = deref(t)
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	tablename, err := tableNameByType(t)
	if err != nil {
		return "", nil, err
	}
	joined := fmt.Sprintf("%s %s ON (t1.id=%s.%s_id)", tablename, tableref, tableref, relindex)
	fields, err := FieldsFor(typeMapper(t).Itemize())
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
func TraversalsByName(fl *FieldList, columns []string) []traversal {
	fields := make([]traversal, len(columns))
	toDo := make([]int, 0, len(columns))
	for idx := range columns {
		toDo = append(toDo, idx)
	}
	for _, fi := range fl.Fields {
		for num, idx := range toDo {
			if fi.Path == columns[idx] ||
				strings.Replace(fi.Path, ".", "_", 1) == columns[idx] {
				fields[idx] = traversal{name: columns[idx], index: fi.Index}
				if relation, ok := fi.Options[OptRelatedTo]; ok {
					for _, item := range fl.Fields {
						if item.Name == relation {
							fields[idx].relation = item.Field
							break
						}
					}
				}
				toDo = append(toDo[0:num], toDo[num+1:]...)
				break
			}
		}
	}
	if len(toDo) > 0 {
		cols := make([]string, len(toDo))
		for id, idx := range toDo {
			cols[id] = columns[idx]
		}
		fmt.Printf("Remaining columns: %v\n", cols)
		fmt.Printf("Fields:\n")
		for _, fi := range fl.Fields {
			fmt.Printf("- %s\n", fi.Path)
		}
	}
	return fields
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
