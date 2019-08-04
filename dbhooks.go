package dmpr

import "database/sql"

func (m *Mapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	m.logger.Infof("DB EXEC: %s with %+v", query, args)
	return m.Conn.Exec(query, args...)
}

func (m *Mapper) NamedExec(query string, arg interface{}) (sql.Result, error) {
	m.logger.Infof("DB NAMED EXEC: %s with %+v", query, arg)
	return m.Conn.NamedExec(query, arg)
}

func (m *Mapper) Get(dest interface{}, query string, args ...interface{}) error {
	m.logger.Infof("DB GET: %s with %+v", query, args)
	return m.Conn.Get(dest, query, args...)
}

func (m *Mapper) Select(dest interface{}, query string, args ...interface{}) error {
	m.logger.Infof("DB SELECT: %s with %+v", query, args)
	return m.Conn.Select(dest, query, args...)
}
