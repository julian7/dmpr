package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx/reflectx"
)

// FieldList stores fields of a reflectx.StructMap's Index (from sqlx), with the structure's type
type FieldList struct {
	Fields []*reflectx.FieldInfo
	Type   reflect.Type
}

// FieldListItem is a line item of a model's field list
type FieldListItem struct {
	reflect.Type
	Field   *reflect.StructField
	Path    string
	Options map[string]string
}

// TypeMap returns a map of types in the form of a StructMap, from the original model's type
func (m *Mapper) TypeMap(t reflect.Type) *FieldList {
	if err := m.tryOpen(); err != nil {
		m.logger.Warnf("cannot get type map of %+v: %v", t, err)
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
			Path:    fi.Path,
			Options: fi.Options,
			Field:   &fieldStruct,
			Type:    fieldStruct.Type,
		})
	}
	return fields
}

// TraversalsByName provides a traversal index for SELECT query results, to map result rows' columns with model's entry positions
func (fl *FieldList) TraversalsByName(columns []string) []traversal {
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
