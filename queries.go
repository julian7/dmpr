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
	fields, err := m.fieldsFor(model, insertType)
	if err != nil {
		return err
	}
	if len(fields) < 1 {
		return errors.New("nothing to create")
	}
	keys := make([]string, 0, len(fields))
	vals := make([]string, 0, len(fields))
	hasID := false
	for _, field := range fields {
		if field.key == "id" {
			hasID = true
			continue
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
	fields, err := m.fieldsFor(model, updateType)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(fields))
	hasID := false
	for _, field := range fields {
		if field.key == "id" {
			hasID = true
			continue
		}
		keys = append(keys, field.val)
	}
	if !hasID {
		return errors.New("no ID field found")
	}
	if len(keys) < 1 {
		return errors.New("nothing to create")
	}
	_, err = m.NamedExec(
		fmt.Sprintf(
			"UPDATE %s SET %s WHERE id=:id",
			tablename,
			strings.Join(keys, ", "),
		),
		model,
	)

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
