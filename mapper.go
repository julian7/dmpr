package dmpr

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"

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

// New sets up a new SQL connection
func New(connString string) *Mapper {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	return &Mapper{url: connString, logger: logger}
}

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

func (m *Mapper) Logger(logger *logrus.Logger) {
	m.logger = logger
}

func (m *Mapper) Name() string {
	return "dbmapper"
}

func (m *Mapper) HealthReport(ctx context.Context) (bool, map[string]string) {
	if m.Conn == nil {
		return false, map[string]string{"error": "database is closed"}
	}
	err := m.Conn.PingContext(ctx)
	if err != nil {
		return false, map[string]string{"error": err.Error()}
	}
	return true, nil
}
