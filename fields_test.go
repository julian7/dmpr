package dmpr

import (
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/julian7/tester"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"
)

type ExampleFieldsForModel struct {
	ID        int64       `db:"id"`
	Name      null.String `db:"name,omitempty"`
	Extra     string      `db:"extra,omitempty"`
	CreatedAt null.Time   `db:"created_at,omitempty"`
	UpdatedAt null.Time   `db:"updated_at,omitempty"`
}

func TestMapper_fieldsFor(t *testing.T) {
	t.Skip("temporarily disabled")
	tests := []struct {
		name     string
		mocks    []func(sqlmock.Sqlmock)
		model    interface{}
		qt       queryType
		expected []queryField
		err      error
	}{
		{
			name: "nil model",
			err:  errors.New("empty model"),
		},
		{
			name:  "empty model for insert",
			model: &ExampleFieldsForModel{},
			qt:    insertType,
			expected: []queryField{
				{key: "id", val: ":id"},
				{key: "created_at", val: "NOW()"},
			},
		},
		{
			name:  "empty model for update",
			model: &ExampleFieldsForModel{Extra: ""},
			qt:    updateType,
			expected: []queryField{
				{key: "id", val: "id=:id"},
				{key: "updated_at", val: "updated_at=NOW()"},
			},
		},
		{
			name: "filled model for insert",
			model: &ExampleFieldsForModel{
				ID:        1,
				Name:      null.StringFrom("Name"),
				CreatedAt: null.TimeFrom(time.Now()),
				Extra:     "Extra",
			},
			qt: insertType,
			expected: []queryField{
				{key: "id", val: ":id"},
				{key: "name", val: ":name"},
				{key: "extra", val: ":extra"},
				{key: "created_at", val: ":created_at"},
			},
		},
		{
			name: "filled model for update",
			model: &ExampleFieldsForModel{
				ID:        1,
				Name:      null.StringFrom("Name"),
				CreatedAt: null.TimeFrom(time.Now()),
				Extra:     "Extra",
			},
			qt: updateType,
			expected: []queryField{
				{key: "id", val: "id=:id"},
				{key: "name", val: "name=:name"},
				{key: "extra", val: "extra=:extra"},
				{key: "created_at", val: "created_at=:created_at"},
				{key: "updated_at", val: "updated_at=NOW()"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()
			for _, item := range tt.mocks {
				item(mock)
			}
			m := &Mapper{
				Conn:   sqlx.NewDb(db, "sqlmock"), // "sqlmock" is a magic string @ sqlmock for driver name
				logger: logrus.New(),
			}
			got, err := m.fieldsFor(tt.model, tt.qt)
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Mapper.fieldsFor() = %v, want %v", got, tt.expected)
			}
		})
	}
}
