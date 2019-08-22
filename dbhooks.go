package dmpr

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// Exec runs sqlx.Exec, with logging
func (m *Mapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB EXEC: %s with %+v", query, args)
	return m.Conn.Exec(query, args...)
}

// NamedExec runs sqlx.NamedExec, with logging
func (m *Mapper) NamedExec(query string, arg interface{}) (sql.Result, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB NAMED EXEC: %s with %+v", query, arg)
	return m.Conn.NamedExec(query, arg)
}

// NamedQuery runs sqlx.NamedQuery, with logging
func (m *Mapper) NamedQuery(query string, arg interface{}) (*sqlx.Rows, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB NAMED QUERY: %s with %+v", query, arg)
	return m.Conn.NamedQuery(query, arg)
}

// Get runs sqlx.Get, with logging
func (m *Mapper) Get(dest interface{}, query string, args ...interface{}) error {
	if err := m.tryOpen(); err != nil {
		return err
	}
	m.logger.Debugf("DB GET: %s with %+v", query, args)
	return m.Conn.Get(dest, query, args...)
}

// Select runs sqlx.Select, with logging
func (m *Mapper) Select(dest interface{}, query string, args ...interface{}) error {
	if err := m.tryOpen(); err != nil {
		return err
	}
	m.logger.Debugf("DB SELECT: %s with %+v", query, args)
	return m.Conn.Select(dest, query, args...)
}

// Queryx runs sqlx.Queryx, with logging
func (m *Mapper) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB QUERYX: %s with %+v", query, args)
	return m.Conn.Queryx(query, args...)
}
