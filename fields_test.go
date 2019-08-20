package dmpr

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/julian7/tester"
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

func Test_FieldsFor(t *testing.T) {
	omitempty := map[string]string{"omitempty": ""}
	tests := []struct {
		name     string
		model    interface{}
		expected []queryField
		err      error
	}{
		{
			name:  "empty model",
			model: &ExampleFieldsForModel{},
			expected: []queryField{
				{key: "id", val: ":id", eq: "id=:id"},
				{key: "name", val: ":name", eq: "name=:name", opts: omitempty},
				{key: "extra", val: ":extra", eq: "extra=:extra", opts: omitempty},
				{key: "created_at", val: ":created_at", eq: "created_at=:created_at", opts: omitempty},
				{key: "updated_at", val: ":updated_at", eq: "updated_at=:updated_at", opts: omitempty},
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
			m := &Mapper{
				Conn:   sqlx.NewDb(db, "sqlmock"), // "sqlmock" is a magic string @ sqlmock for driver name
				logger: logrus.New(),
			}
			got, err := FieldsFor(m.TypeMap(TypeOf(tt.model)).Itemize())
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}

			for idx := range tt.expected {
				if len(tt.expected[idx].opts) == 0 {
					tt.expected[idx].opts = map[string]string{}
				}
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("FieldsFor() = %v, want %v", got, tt.expected)
			}
		})
	}
}
