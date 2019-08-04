package dmpr

import (
	"fmt"
	"reflect"
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
		fmt.Sprintf("SELECT * FROM %s WHERE id = ?", table),
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
	fields := m.fieldMap(model)
	keys := make([]string, 0, len(fields))
	idfield := ""
	for k := range fields {
		if k == "id" {
			idfield = k
		} else {
			keys = append(keys, k)
		}
	}
	if len(keys) < 1 {
		return errors.New("nothing to create")
	}
	result, err := m.Exec(
		fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (:%s)",
			tablename,
			strings.Join(keys, ", "),
			strings.Join(keys, ", :"),
		),
		model,
	)
	if err == nil {
		lastid, err := result.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "requesting last insert ID")
		}
		val := reflect.ValueOf(model)
		_ = val
		val.FieldByName(idfield).Set(reflect.ValueOf(lastid))
	}
	return err
}

// Update inserts an item into the database
func (m *Mapper) Update(model interface{}) error {
	tablename, err := tableName(model)
	if err != nil {
		return err
	}
	fields := m.fieldMap(model)
	keys := make([]string, 0, len(fields))
	idfield := ""
	for k := range fields {
		field := fmt.Sprintf("%s=:%s", k, k)
		if k == "id" {
			idfield = field
		} else {
			keys = append(keys, field)
		}
	}
	if idfield == "" {
		return errors.New("no ID field found")
	}
	if len(keys) < 1 {
		return errors.New("nothing to create")
	}
	_, err = m.NamedExec(
		fmt.Sprintf(
			"UPDATE %s SET %s WHERE %s",
			tablename,
			strings.Join(keys, ", "),
			idfield,
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
			"DELETE FROM %s WHERE id = ?",
			tablename,
		),
		id,
	)
	return err
}
