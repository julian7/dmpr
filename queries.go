package dmpr

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Find searches database for a row by ID
func (m *Mapper) Find(model interface{}, id int64) error {
	table, err := tableName(model)
	if err != nil {
		return err
	}
	return m.Get(
		model,
		fmt.Sprintf("SELECT * FROM %s WHERE id = $1", table),
		id,
	)
}

// FindBy searches database for a row by a column match
func (m *Mapper) FindBy(model interface{}, column string, needle string) error {
	table, err := tableName(model)
	if err != nil {
		return err
	}
	return m.Get(
		model,
		fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", table, column),
		needle,
	)
}

// All returns all elements into an array of models
func (m *Mapper) All(models interface{}) error {
	table, err := tableName(models)
	if err != nil {
		return err
	}
	return m.Select(
		models,
		fmt.Sprintf("SELECT * FROM %s", table),
	)
}

// Create inserts an item into the database
func (m *Mapper) Create(model interface{}) error {
	tablename, err := tableName(model)
	if err != nil {
		return err
	}
	fields, err := FieldsFor(m.TypeMap(TypeOf(model)).Itemize())
	if err != nil {
		return err
	}
	if len(fields) < 1 {
		return errors.New("nothing to create")
	}
	keys := make([]string, 0, len(fields))
	vals := make([]string, 0, len(fields))
	fieldmap := m.FieldMap(model)
	hasID := false
	for _, field := range fields {
		fieldVal, ok := fieldmap[field.key]
		if !ok {
			return errors.Errorf("unknown field key: %s", field.key)
		}
		if field.key == "id" {
			hasID = true
			continue
		}
		if _, ok := field.opts["omitempty"]; ok && isEmptyValue(fieldVal) {
			if field.key != "created_at" {
				continue
			}
			field.val = "NOW()"
		}
		keys = append(keys, field.key)
		vals = append(vals, field.val)
	}
	rows, err := m.NamedQuery(
		fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)%s",
			tablename,
			strings.Join(keys, ", "),
			strings.Join(vals, ", "),
			map[bool]string{true: " RETURNING id", false: ""}[hasID],
		),
		model,
	)
	if err == nil {
		if hasID {
			rows.Next()
			err = rows.StructScan(model)
			if err != nil {
				return err
			}
		}
	}
	return err
}

// Update inserts an item into the database
func (m *Mapper) Update(model interface{}) error {
	tablename, err := tableName(model)
	if err != nil {
		return err
	}
	fields, err := FieldsFor(m.TypeMap(TypeOf(model)).Itemize())
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(fields))
	fieldmap := m.FieldMap(model)
	hasID := false
	hasUpdatedAt := true
	for _, field := range fields {
		fieldVal, ok := fieldmap[field.key]
		if !ok {
			return errors.Errorf("unknown field key: %s", field.key)
		}
		if field.key == "id" {
			hasID = true
			continue
		}
		if _, ok := field.opts["omitempty"]; ok && isEmptyValue(fieldVal) {
			if field.key != "updated_at" {
				continue
			}
			hasUpdatedAt = true
			field.eq = "updated_at=NOW()"
		}
		keys = append(keys, field.eq)
	}
	if !hasID {
		return errors.New("no ID field found")
	}
	if len(keys) < 1 {
		return errors.New("nothing to create")
	}
	rows, err := m.NamedQuery(
		fmt.Sprintf(
			"UPDATE %s SET %s WHERE id=:id%s",
			tablename,
			strings.Join(keys, ", "),
			map[bool]string{true: " RETURNING updated_at", false: ""}[hasUpdatedAt],
		),
		model,
	)
	if err == nil && hasUpdatedAt {
		rows.Next()
		err = rows.StructScan(model)
	}
	return err
}

// Delete deletes a row
func (m *Mapper) Delete(model interface{}, id int64) error {
	tablename, err := tableName(model)
	if err != nil {
		return err
	}
	_, err = m.Exec(
		fmt.Sprintf(
			"DELETE FROM %s WHERE id = $1",
			tablename,
		),
		id,
	)
	return err
}
