package dmpr

import (
	"fmt"
	"strings"
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
	query, args, err := q.allSelector()
	if err != nil {
		return nil
	}
	return q.mapper.Select(q.model, query, args)
}

func (q *SelectQuery) allSelector() (string, map[string]interface{}, error) {
	var args map[string]interface{}
	table, err := tableName(q.model)
	if err != nil {
		return "", args, err
	}
	selected, joined, err := q.getSelect()
	if err != nil {
		return "", args, err
	}
	joined = append([]string{table + " t1"}, joined...)
	whereClause := ""
	if q.where != nil {
		whereClause = fmt.Sprintf(" WHERE %s", q.where.Where(true))
		args = q.where.Value()
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
	fields, err := q.mapper.fieldsFor(q.model, selectType)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range fields {
		selected = append(selected, "t1."+item.key)
	}

	if len(q.incl) > 0 {
		for idx, incl := range q.incl {
			fields, err := q.mapper.relatedFieldsFor(q.model, incl, selectType)
			if err != nil {
				return nil, joined, err
			}
			joinTable, err := q.mapper.subTableName(q.model, incl)
			if err != nil {
				return nil, joined, err
			}
			tableref := fmt.Sprintf("t%d", idx+2)
			joined = append(joined, fmt.Sprintf("%s %s ON (t1.%s_id=%s.id)", joinTable, tableref, incl, tableref))
			for _, field := range fields {
				if field.key == "id" {
					continue
				}
				selected = append(selected, fmt.Sprintf("t%d.%s AS %s_%s", idx+2, field.key, incl, field.key))
			}
		}
	}
	return selected, joined, nil
}
