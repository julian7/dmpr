package dmpr

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// Exec runs sqlx.Exec nicely. It opens database if needed, and logs the query.
func (m *Mapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB EXEC: %s with %+v", query, args)
	return m.Conn.Exec(query, args...)
}

// NamedExec runs sqlx.NamedExec nicely. It opens database if needed, and logs the query.
func (m *Mapper) NamedExec(query string, arg interface{}) (sql.Result, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB NAMED EXEC: %s with %+v", query, arg)
	return m.Conn.NamedExec(query, arg)
}

// NamedQuery runs sqlx.NamedQuery nicely. It opens database if needed, and logs the query.
func (m *Mapper) NamedQuery(query string, arg interface{}) (*sqlx.Rows, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB NAMED QUERY: %s with %+v", query, arg)
	return m.Conn.NamedQuery(query, arg)
}

// Get runs sqlx.Get nicely. It opens database if needed, and logs the query.
func (m *Mapper) Get(dest interface{}, query string, args ...interface{}) error {
	if err := m.tryOpen(); err != nil {
		return err
	}
	m.logger.Debugf("DB GET: %s with %+v", query, args)
	return m.Conn.Get(dest, query, args...)
}

// Select runs sqlx.Select nicely. It opens database if needed, and logs the query.
func (m *Mapper) Select(dest interface{}, query string, args ...interface{}) error {
	if err := m.tryOpen(); err != nil {
		return err
	}
	m.logger.Debugf("DB SELECT: %s with %+v", query, args)
	return m.Conn.Select(dest, query, args...)
}

// Queryx runs sqlx.Queryx nicely. It opens database if needed, and logs the query.
func (m *Mapper) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	if err := m.tryOpen(); err != nil {
		return nil, err
	}
	m.logger.Debugf("DB QUERYX: %s with %+v", query, args)
	return m.Conn.Queryx(query, args...)
}
