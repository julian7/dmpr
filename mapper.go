package dmpr

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"reflect"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	// PGSQL implementation
	_ "github.com/lib/pq"
)

// Mapper is our PGSQL connection struct
type Mapper struct {
	Conn   *sqlx.DB
	url    string
	logger *logrus.Logger
}

// New sets up a new SQL connection. It sets up a "black hole" logger too.
func New(connString string) *Mapper {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	return &Mapper{url: connString, logger: logger}
}

// Open opens connection to the database. It is implicitly called by
// db calls defined in Mapper, but sometimes it's desirable to make
// sure the database is available at start time.
func (m *Mapper) Open() error {
	connURL, err := url.Parse(m.url)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %v", m.url, err)
	}
	driver := connURL.Scheme
	db, err := sqlx.Open(driver, m.url)
	if err != nil {
		return err
	}
	m.Conn = db
	connURL.User = url.UserPassword("REDACTED", "REDACTED")
	m.logger.Infof("dbmapper connected to underlying database: %v", connURL.String())
	return nil
}

func (m *Mapper) tryOpen() error {
	if m.Conn != nil {
		return nil
	}
	return m.Open()
}

// Logger sets up internal log method, replacing the discarding logger.
func (m *Mapper) Logger(logger *logrus.Logger) {
	m.logger = logger
}

// FieldMap returns a map of fields for a model. It handles pointer of model.
func (m *Mapper) FieldMap(model interface{}) map[string]reflect.Value {
	if err := m.tryOpen(); err != nil {
		m.logger.Warnf("cannot get field map of %+v: %v", model, err)
		return map[string]reflect.Value{}
	}
	return m.Conn.Mapper.FieldMap(indirect(reflect.ValueOf(model)))
}

// Name returns module name. Used for subsystem health checks.
func (m *Mapper) Name() string {
	return "dbmapper"
}

// HealthReport returns healthy status, or map of issues. Currently,
// a closed database is reported as an error.
func (m *Mapper) HealthReport(ctx context.Context) (healthy bool, errors map[string]string) {
	if m.Conn == nil {
		return false, map[string]string{"error": "database is closed"}
	}
	err := m.Conn.PingContext(ctx)
	if err != nil {
		return false, map[string]string{"error": err.Error()}
	}
	return true, nil
}
