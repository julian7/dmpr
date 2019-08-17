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

func (m *Mapper) NewSelect(model interface{}) *SelectQuery {
	return &SelectQuery{
		mapper: m,
		model:  model,
	}
}

func (q *SelectQuery) Select(selectors ...string) *SelectQuery {
	if len(q.sel) < 1 {
		q.sel = make([]string, 0, len(selectors))
	}
	q.sel = append(q.sel, selectors...)
	return q
}

func (q *SelectQuery) Include(selectors ...string) *SelectQuery {
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
		return nil
	}
	rows, err := q.mapper.Queryx(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	fields := TraversalsByName(q.mapper.TypeMap(t), columns)

	values := make([]interface{}, len(columns))
	for rows.Next() {
		vp := reflect.New(t)
		v := reflect.Indirect(vp)

		err = fieldsByTraversal(v, fields, values, true)
		if err != nil {
			return err
		}
		err = rows.Scan(values...)
		if err != nil {
			return err
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
	fieldlist := FieldList(structmap)
	fields, err := FieldsFor(fieldlist)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range fields {
		selected = append(selected, "t1."+item.key)
	}

	if len(q.incl) > 0 {
		for idx, incl := range q.incl {
			fields, err := RelatedFieldsFor(fieldlist, incl)
			if err != nil {
				return nil, joined, err
			}
			joinTable, err := SubTableName(fieldlist, incl)
			if err != nil {
				return nil, joined, err
			}
			tableref := fmt.Sprintf("t%d", idx+2)
			joined = append(joined, fmt.Sprintf("%s %s ON (t1.%s_id=%s.id)", joinTable, tableref, incl, tableref))
			for _, field := range fields {
				selected = append(selected, fmt.Sprintf("t%d.%s AS %s_%s", idx+2, field.key, incl, field.key))
			}
		}
	}
	return selected, joined, nil
}
