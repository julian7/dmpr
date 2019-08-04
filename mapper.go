package dmpr

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	// PGSQL implementation
	_ "github.com/lib/pq"
)

// Mapper is our PGSQL connection struct
type Mapper struct {
	Conn   *sqlx.DB
	logger *logrus.Logger
}

// New sets up a new SQL connection
func New(connString string, logger *logrus.Logger) (*Mapper, error) {
	connURL, err := url.Parse(connString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %v", connString, err)
	}
	driver := connURL.Scheme
	db, err := sqlx.Open(driver, connString)
	if err != nil {
		return nil, err
	}
	conn := &Mapper{Conn: db, logger: logger}
	return conn, nil
}

func (m *Mapper) Name() string {
	return "dbmapper"
}

func (m *Mapper) HealthReport(ctx context.Context) (bool, map[string]string) {
	err := m.Conn.PingContext(ctx)
	if err != nil {
		return false, map[string]string{"error": err.Error()}
	}
	return true, nil
}
