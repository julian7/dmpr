package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

const (
	// OptRelatedTo is an internal struct tag option mapper inserts for all columns, which is a subfield of an embedded table. Its value is the name of the embedded table.
	OptRelatedTo = "_related_to_"
	// OptUnrelated is an internal struct tag option mapper inserts for all columns, which have parents, but they are not a subfield of a known relation.
	OptUnrelated = "_unrelated_"
	// OptBelongs is a struct tag option marking a "belongs-to" relation.
	OptBelongs = "belongs"
	// OptRelation is a struct tag option marking a "has-one", "has-many", or a "many-to-many" relation, containing the other end's ref stub for the struct's ID.
	//
	// Example tag: `db:"posts,relation=author"`: you can reference this join as `posts`, and the other end will have a field called `author_id` to reference this table.
	OptRelation = "relation"
	// OptReverse is a struct tag option marking a "many-to-many" relation,
	// containing the stub how the other end is referenced in the linker table (see OptThrough).
	// OptReverse is taken into consideration only if OptRelation and OptThrough
	// are provided.
	//
	// Example tag: `db:"groups,relation=user,reverse=group,through=user_groups"`:
	// you can reference this join as `groups`, and there must be a `user_groups`
	// table with a `user_id` and a `group_id` field. `user_id` references to this
	// table, `group_id` references to the joined table.
	OptReverse = "reverse"
	// OptThrough is a struct tag option marking a "many-to-many" relation,
	// containing the linker table's name. See OptReverse for an example.
	// OptThrough is taken into consideration only if OptRelation and OptReverse
	// are also provided.
	OptThrough = "through"
)

// FieldList stores fields of a reflectx.StructMap's Index (from sqlx), with the structure's type
type FieldList struct {
	Fields []FieldListItem
	Type   reflect.Type
	Joins  map[string]*FieldList
}

// FieldListItem is a line item of a model's field list
type FieldListItem struct {
	reflect.Type
	Field     reflect.StructField
	Index     []int
	Name      string
	Options   map[string]string
	Path      string
	Traversed bool
}

type queryField struct {
	key  string
	val  string
	eq   string
	opts map[string]string
}

// Traversal stores dial information for each column into the resulting model instance
type Traversal struct {
	Name     string
	Index    []int
	Relation reflect.StructField
}

type Traversals []*Traversal

// FieldList returns a map of types in the form of a StructMap, from the original model's type
func (m *Mapper) FieldList(t reflect.Type) *FieldList {
	if err := m.tryOpen(); err != nil {
		m.logger.Warnf("cannot get type map of %+v: %v", t, err)
		return nil
	}
	t = deref(t)
	fieldList := &FieldList{Type: t}
	fl := m.Conn.Mapper.TypeMap(t).Index
	related := []string{}
	for _, fi := range fl {
		if _, ok := fi.Options[OptBelongs]; ok {
			related = append(related, fi.Path)
		}
	}
	fieldList.Fields = make([]FieldListItem, 0, len(fl))
	for _, fi := range fl {
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
				fi.Options[OptUnrelated] = ""
			}
		}
		fieldStruct := t.FieldByIndex(fi.Index)
		fieldList.Fields = append(fieldList.Fields, FieldListItem{
			Field:     fieldStruct,
			Index:     fi.Index,
			Name:      fi.Name,
			Options:   fi.Options,
			Path:      strings.ReplaceAll(fi.Path, ".", "_"),
			Traversed: false,
			Type:      fieldStruct.Type,
		})
	}
	return fieldList
}

// FieldsFor converts FieldListItems to query fields SQL query builders can use. It doesn't include related fields.
func (fl *FieldList) FieldsFor() ([]queryField, error) {
	queryFields := make([]queryField, 0, len(fl.Fields))
FieldsForLoop:
	for _, fi := range fl.Fields {
		for _, item := range []string{OptRelatedTo, OptUnrelated} {
			if _, ok := fi.Options[item]; ok {
				continue FieldsForLoop
			}
		}
		field := fi.QField()
		if field != nil {
			queryFields = append(queryFields, *field)
		}
	}
	return queryFields, nil
}

// RelatedFieldsFor converts FieldListItems to JOINs and SELECTs SQL query builders can use directly
func (fl *FieldList) RelatedFieldsFor(relation, tableref string, cb func(reflect.Type) *FieldList) (joins []string, selects []string, err error) {
	for _, field := range fl.Fields {
		if field.Path == relation {
			if _, ok := field.Options[OptRelation]; ok {
				return fl.HasNFieldsFor(relation, tableref, field, cb)
			}
			tablename, err := tableNameByType(field.Type)
			if err != nil {
				return nil, nil, err
			}
			return fl.BelongsToFieldsFor(relation, tableref, tablename)
		}
	}
	return nil, nil, errors.Errorf("Relation %q not found", relation)
}

// BelongsToFieldsFor converts FieldListItems to JOIN and SELECTs query substrings SQL query buildders can use directly
func (fl *FieldList) BelongsToFieldsFor(relation, tableref, tablename string) ([]string, []string, error) {
	joined := []string{fmt.Sprintf("%s %s ON (t1.%s_id=%s.id)", tablename, tableref, relation, tableref)}
	selected := []string{}
	rel := len(relation) + 1
FieldScan:
	for _, fi := range fl.Fields {
		for _, option := range []string{OptUnrelated, OptRelation, OptBelongs} {
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
// It uses a callback, which can provide a *FieldList from the referenced type.
func (fl *FieldList) HasNFieldsFor(relation, tableref string, field FieldListItem, typeMapper func(reflect.Type) *FieldList) ([]string, []string, error) {
	var joined []string
	relindex, hasRelIndex := field.Options[OptRelation]
	revindex, hasRevIndex := field.Options[OptReverse]
	throughTable, hasThrough := field.Options[OptThrough]

	if !hasRelIndex {
		return nil, nil, errors.New("not a relation")
	}

	t := deref(field.Type)
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	tablename, err := tableNameByType(t)
	if err != nil {
		return nil, nil, err
	}
	if hasRevIndex && hasThrough {
		joined = append(
			joined,
			fmt.Sprintf("%s t%s ON (t1.id=t%s.%s_id)", throughTable, tableref, tableref, relindex),
			fmt.Sprintf("%s %s ON (%s.id=t%s.%s_id)", tablename, tableref, tableref, tableref, revindex),
		)
	} else {
		joined = append(joined, fmt.Sprintf("%s %s ON (t1.id=%s.%s_id)", tablename, tableref, tableref, relindex))
	}
	flSub := typeMapper(t)
	fields, err := flSub.FieldsFor()
	if err != nil {
		return nil, nil, err
	}
	if len(fl.Joins) == 0 {
		fl.Joins = map[string]*FieldList{}
	}
	fl.Joins[relation] = flSub
	selected := make([]string, 0, len(fields))
	for _, field := range fields {
		selected = append(selected, fmt.Sprintf("%s.%s AS %s_%s", tableref, field.key, relation, field.key))
	}
	return joined, selected, nil
}

// TraversalsByName provides a traversal index for SELECT query results, to map result rows' columns with model's entry positions
func (fl *FieldList) TraversalsByName(columns []string) (Traversals, error) {
	fields := make([]*Traversal, len(columns))
	for idx := range columns {
		trav := fl.traversalByName(columns[idx], "", nil)
		if trav == nil {
			return nil, errors.Errorf("can't find column %q in model struct", columns[idx])
		}
		fields[idx] = trav
	}
	return fields, nil
}

func (fl *FieldList) traversalByName(column, prefix string, parentIndex []int) *Traversal {
	subcol := strings.SplitN(column, "_", 2)
	if len(parentIndex) < 1 {
		parentIndex = []int{}
	}
	for idx, fi := range fl.Fields {
		if fi.Traversed {
			continue
		}
		if fi.Path == column {
			fl.Fields[idx].Traversed = true
			parentIndex = append(parentIndex, fi.Index...)
			trav := &Traversal{Name: prefix + column, Index: parentIndex}
			if relation, ok := fi.Options[OptRelatedTo]; ok {
				for _, item := range fl.Fields {
					if item.Name == relation {
						trav.Relation = item.Field
					}
				}
			}
			return trav
		}
		if len(subcol) > 1 && fi.Path == subcol[0] {
			otherfl, ok := fl.Joins[subcol[0]]
			if !ok {
				continue
			}
			trav := otherfl.traversalByName(subcol[1], subcol[0]+"_", fi.Index)
			if trav == nil {
				continue
			}
			return trav
		}
	}
	return nil
}

// QField returns a query field based on a FieldListItem
func (fi FieldListItem) QField() *queryField {
	val := ":" + fi.Path
	for _, opt := range []string{"relation", "belongs"} {
		_, ok := fi.Options[opt]
		if ok {
			return nil
		}
	}
	field := &queryField{}
	field.key = fi.Path
	field.opts = fi.Options
	field.val = val
	field.eq = fi.Path + "=" + val
	return field
}

// Map creates new empty model struct instance, and fills values slice with
// pointers to model elements by traversal indexes. This makes sqlx.Scan
// load rows directly into model instances.
func (t Traversals) Map(v reflect.Value, values []interface{}) error {
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return errors.New("argument not a struct")
	}

	for i, traversal := range t {
		if len(traversal.Index) == 0 {
			values[i] = new(interface{})
			continue
		}
		f := fieldByIndexes(v, traversal.Index)
		values[i] = f.Addr().Interface()
	}
	return nil
}
