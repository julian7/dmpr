package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
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
	t := deref(field.Type)
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	tablename, err := tableNameByType(t)
	if err != nil {
		return nil, nil, err
	}
	relindex, _ := field.Options[OptRelation]
	revindex, hasRevIndex := field.Options[OptReverse]
	throughTable, hasThrough := field.Options[OptThrough]
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
func (fl *FieldList) TraversalsByName(columns []string) ([]*traversal, error) {
	fields := make([]*traversal, len(columns))
	for idx := range columns {
		trav, err := fl.TraversalByName(columns[idx])
		if err != nil {
			return nil, err
		}
		fields[idx] = trav
	}
	return fields, nil
}

func (fl *FieldList) TraversalByName(column string) (*traversal, error) {
	subcol := strings.SplitN(column, "_", 2)
	for idx, fi := range fl.Fields {
		if fi.Traversed {
			continue
		}
		if fi.Path == column {
			fl.Fields[idx].Traversed = true
			trav := &traversal{name: column, index: fi.Index}
			if relation, ok := fi.Options[OptRelatedTo]; ok {
				for _, item := range fl.Fields {
					if item.Name == relation {
						trav.relation = item.Field
					}
				}
			}
			return trav, nil
		}
		if fi.Path == subcol[0] {
			otherfl, ok := fl.Joins[subcol[0]]
			if len(subcol) > 1 && ok {
				trav, err := otherfl.TraversalByName(subcol[1])
				if err != nil {
					continue
				}
				trav.name = column
				oldidx := make([]int, len(fi.Index))
				copy(oldidx, fi.Index)
				trav.index = append(oldidx, trav.index...)
				return trav, nil
			}
		}
	}
	return nil, errors.Errorf("can't find column %q in model struct", column)
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
