package dmpr

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

type SelectQuery struct {
	mapper *Mapper
	model  interface{}
	sel    []string
	incl   []string
	where  Operator
}

func (m *Mapper) NewSelect(model interface{}) (*SelectQuery, error) {
	_, err := tableName(model)
	if err != nil {
		return nil, err
	}
	return &SelectQuery{
		mapper: m,
		model:  model,
	}, nil
}

func (q *SelectQuery) Select(selectors ...string) *SelectQuery {
	if len(q.sel) < 1 {
		q.sel = make([]string, 0, len(selectors))
	}
	q.sel = append(q.sel, selectors...)
	return q
}

func (q *SelectQuery) Join(selectors ...string) *SelectQuery {
	if len(q.incl) < 1 {
		q.incl = make([]string, 0, len(selectors))
	}
	q.incl = append(q.incl, selectors...)
	return q
}

func (q *SelectQuery) Where(op Operator) *SelectQuery {
	if q.where != nil {
		q.where = And(q.where, op)
	} else {
		q.where = op
	}
	return q
}

func (q *SelectQuery) All() error {
	value := reflect.ValueOf(q.model)

	if value.Kind() != reflect.Ptr {
		return errors.New("pointer is expected as Select destination")
	}
	if value.IsNil() {
		return errors.New("nil pointer passed to Select destination")
	}
	value = reflect.Indirect(value)
	t := deref(value.Type())
	if t.Kind() != reflect.Slice {
		return errors.New("pointer to slice is expected as Select destination")
	}
	t = deref(t.Elem())

	query, args, err := q.allSelector()
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
	fields, err := q.mapper.TypeMap(t).TraversalsByName(columns)
	if err != nil {
		return errors.Wrap(err, "SelectAll traversal")
	}

	values := make([]interface{}, len(columns))
	for rows.Next() {
		vp := reflect.New(t)
		v := reflect.Indirect(vp)

		err = fieldsByTraversal(v, fields, values, true)
		if err != nil {
			return errors.Wrap(err, "SelectAll traversal")
		}
		err = rows.Scan(values...)
		if err != nil {
			return errors.Wrap(err, "SelectAll scan")
		}
		value.Set(reflect.Append(value, v))
	}
	return rows.Err()
}

func (q *SelectQuery) allSelector() (string, []interface{}, error) {
	table, err := tableName(q.model)
	if err != nil {
		return "", nil, err
	}
	selected, joined, err := q.getSelect()
	if err != nil {
		return "", nil, err
	}
	joined = append([]string{table + " t1"}, joined...)
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

func (q *SelectQuery) getSelect() ([]string, []string, error) {
	var selected []string
	var joined []string
	if len(q.sel) >= 1 {
		return q.sel, joined, nil
	}
	structmap := q.mapper.TypeMap(TypeOf(q.model))
	fields, err := structmap.FieldsFor()
	if err != nil {
		return nil, nil, err
	}
	for _, item := range fields {
		selected = append(selected, "t1."+item.key)
	}

	if len(q.incl) > 0 {
		for idx, incl := range q.incl {
			tableref := fmt.Sprintf("t%d", idx+2)
			joining, selecting, err := structmap.RelatedFieldsFor(incl, tableref, func(t reflect.Type) *FieldList {
				return q.mapper.TypeMap(t)
			})
			if err != nil {
				return nil, joined, err
			}
			joined = append(joined, joining)
			selected = append(selected, selecting...)
		}
	}
	return selected, joined, nil
}
