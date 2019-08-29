package dmpr

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
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

type queryField struct {
	key  string
	val  string
	eq   string
	opts map[string]string
}

// Reflect dissects provided model reference into a type and value for further inspection.
// It accepts pointer, slice, and pointer of slices indirections.
func Reflect(model interface{}) (reflect.Type, reflect.Value) {
	value := indirect(reflect.ValueOf(model))
	t := value.Type()
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	return t, value
}

// FieldMap returns a map of fields for a model. It handles pointer of model.
func (m *Mapper) FieldMap(model interface{}) map[string]reflect.Value {
	if err := m.tryOpen(); err != nil {
		log.Warnf("cannot get field map of %+v: %v", model, err)
		return map[string]reflect.Value{}
	}
	return m.Conn.Mapper.FieldMap(indirect(reflect.ValueOf(model)))
}

type traversal struct {
	name     string
	index    []int
	relation reflect.StructField
}

// fieldByIndexes dials in to a value by index #s, returning the value inside. It allocates pointers and maps when needed.
func fieldByIndexes(v reflect.Value, indexes []int) reflect.Value {
	for _, i := range indexes {
		v = reflect.Indirect(v)
		if v.Kind() == reflect.Slice {
			if v.Len() > 0 {
				v = indirect(v.Index(0))
			} else {
				t := v.Type().Elem()
				newVal := reflect.New(deref(t))
				v.Set(reflect.Append(v, newVal))
				v = reflect.Indirect(newVal)
			}
		}
		v = v.Field(i)
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
func fieldsByTraversal(v reflect.Value, traversals []*traversal, values []interface{}, ptrs bool) error {
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

// mergeFields merges a struct's field's slice values into another one
func mergeFields(dst, src reflect.Value) (reflect.Value, error) {
	dst = reflect.Indirect(dst)
	src = reflect.Indirect(src)
	if dst.Kind() != reflect.Struct {
		return dst, errors.New("dst is not a struct")
	}
	if src.Type() != dst.Type() {
		return dst, errors.New("src is not of the same type")
	}
	for idx := 0; idx < dst.NumField(); idx++ {
		dstField := dst.Field(idx)
		srcField := src.Field(idx)
		if dstField.Kind() == reflect.Slice && dstField.Type().Elem() == srcField.Type().Elem() {
			dstField.Set(reflect.AppendSlice(dstField, srcField))
		}
	}

	return dst, nil
}
