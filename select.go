package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

// SelectQuery represent a new SELECT query builder
type SelectQuery struct {
	mapper *Mapper
	model  interface{}
	sel    []string
	incl   []string
	where  Operator
}

// NewSelect returns a new SelectQuery with the provided model attached
func (m *Mapper) NewSelect(model interface{}) (*SelectQuery, error) {
	_, err := tableName(model)
	if err != nil {
		return nil, err
	}
	value := reflect.ValueOf(model)

	if value.Kind() != reflect.Ptr {
		return nil, errors.New("pointer is expected as Select destination")
	}
	if value.IsNil() {
		return nil, errors.New("nil pointer passed to Select destination")
	}
	value = reflect.Indirect(value)
	t := deref(value.Type())
	if t.Kind() != reflect.Slice {
		return nil, errors.New("pointer to slice is expected as Select destination")
	}
	return &SelectQuery{
		mapper: m,
		model:  model,
	}, nil
}

// Select sets columns to be selected into model. By default, all fields
// in the model and its joined relations are populated.
func (q *SelectQuery) Select(selectors ...string) *SelectQuery {
	if len(q.sel) < 1 {
		q.sel = make([]string, 0, len(selectors))
	}
	q.sel = append(q.sel, selectors...)
	return q
}

// Join prepares query for joining tables, which will populate referenced
// submodels. This supports "has one," "has many," and "many to many"
// relations. Parameters have to reference db field names, which are also
// some sort of relations. Examples:
//
// ```golang
// type Table struct {
//     ID           int
//     WeBelongToID int             `db:"we_belong_to_id"`
//     WeBelongTo   TableWeBelongTo `db:"we_belong_to,belongs"`
//     HasOne       OtherTable      `db:"has_one,relation=table"`
//     HasMany      []*ManyTable    `db:"has_many,relation=table"`
//     ManyToMany   []*M2MTable     `db:"many_to_many,relation=table,reverse=mtm,through=table_m2ms"`
// }
// ```
//
// represents four join possibilities: "we_belong_to," "has_one," and "has_many."
// In the first example, we expect the other item's ID stored with the name of
// the relation + "_id" ("we_belong_to_id"), In the other two examples, we
// expect our ID to be stored at the other end with the name of the value of
// "relation" optional tag + "_id" ("table_id").
// In the last example, we also expect that the relation is set up in
// "table_m2ms" table, and it also has a has many relationship with M2MTable,
// referencing its ID as "reverse" optional tag + "_id" ("mtm_id") in
// the "through" table ("table_m2ms") too.
func (q *SelectQuery) Join(selectors ...string) *SelectQuery {
	if len(q.incl) < 1 {
		q.incl = make([]string, 0, len(selectors))
	}
	q.incl = append(q.incl, selectors...)
	return q
}

// Where sets where clauses to the SELECT query, using Operator interface.
// Calling it multiple times will yield an AND relationship among operators.
func (q *SelectQuery) Where(op Operator) *SelectQuery {
	if q.where != nil {
		q.where = And(q.where, op)
	} else {
		q.where = op
	}
	return q
}

// All executes SELECT query, returning all the items selected. This call
// evaluates provided parameters, builds SQL query, and populates model
// slice the SelectQuery is created with.
func (q *SelectQuery) All() error {
	t, value := Reflect(q.model)
	fl := q.mapper.FieldList(t)

	query, args, err := q.allSelector(fl)
	if err != nil {
		return err
	}
	rows, err := q.mapper.Queryx(query, args...)
	if err != nil {
		return errors.Wrap(err, "SelectAll query")
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return errors.Wrap(err, "SelectAll columns")
	}
	fields, err := fl.TraversalsByName(columns)
	if err != nil {
		return errors.Wrap(err, "SelectAll traversal")
	}
	indexindex := -1
	for idx := range columns {
		if columns[idx] == "id" {
			indexindex = idx
			break
		}
	}
	values := make([]interface{}, len(columns))
	index := map[int]int{}
	rowNum := 0
	for rows.Next() {
		vp := reflect.New(t)
		v := reflect.Indirect(vp)

		if err := fields.Map(v, values); err != nil {
			return errors.Wrap(err, "SelectAll traversal mapping")
		}
		if err := rows.Scan(values...); err != nil {
			return errors.Wrap(err, "SelectAll scan")
		}
		if indexindex >= 0 {
			thisid, ok := values[indexindex].(*int)
			if ok {
				if otherRow, ok := index[*thisid]; ok {
					updatedRow, err := mergeFields(value.Index(otherRow), v)
					if err != nil {
						return errors.Wrap(err, "SelectAll merging fields")
					}
					value.Index(otherRow).Set(updatedRow)
					continue
				}
				index[*thisid] = rowNum
			}
		}
		value.Set(reflect.Append(value, v))
		rowNum++
	}
	return rows.Err()
}

func (q *SelectQuery) allSelector(fl *FieldList) (string, []interface{}, error) {
	table, err := tableName(q.model)
	if err != nil {
		return "", nil, err
	}
	var selected []string
	joined := []string{table + " t1"}
	if len(q.sel) >= 1 {
		selected = q.sel
	} else {
		fields, err := fl.FieldsFor()
		if err != nil {
			return "", nil, err
		}
		for _, item := range fields {
			selected = append(selected, "t1."+item.key)
		}

		if len(q.incl) > 0 {
			j, s, err := handleJoins(fl, q.incl, func(ref, tableref string) ([]string, []string, error) {
				return fl.RelatedFieldsFor(ref, tableref, func(t reflect.Type) *FieldList {
					return q.mapper.FieldList(t)
				})
			})
			if err != nil {
				return "", nil, err
			}
			joined = append(joined, j...)
			selected = append(selected, s...)
		}
	}
	whereClause := ""
	args := []interface{}{}
	if q.where != nil {
		whereClause = fmt.Sprintf(" WHERE %s", q.where.Where(true))
		values := q.where.Values()
		for _, val := range q.where.Keys() {
			args = append(args, values[val])
		}
	}
	return fmt.Sprintf("SELECT %s "+
		"FROM %s%s",
		strings.Join(selected, ", "),
		strings.Join(joined, " LEFT JOIN "),
		whereClause,
	), args, nil
}

func handleJoins(fl *FieldList, joins []string, fielder func(string, string) ([]string, []string, error)) ([]string, []string, error) {
	var joined, selected []string
	for idx, incl := range joins {
		tableref := fmt.Sprintf("t%d", idx+2)
		joining, selecting, err := fielder(incl, tableref)
		if err != nil {
			return nil, nil, err
		}
		joined = append(joined, joining...)
		selected = append(selected, selecting...)
	}
	return joined, selected, nil
}
