package dmpr

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/julian7/tester"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ExampleModel struct {
	ID   int64 `db:"id"`
	Name string
}

func TestMapper_Find(t *testing.T) {
	tests := []struct {
		name     string
		mocks    []func(sqlmock.Sqlmock)
		model    interface{}
		id       int64
		expected interface{}
		err      error
	}{
		{
			name: "nil model",
			id:   5,
			err:  errors.New("Invalid Model Type"),
		},
		{
			name:  "unexported model",
			model: struct{}{},
			id:    5,
			err:   errors.New("Invalid Model Type"),
		},
		{
			name: "normal query",
			mocks: []func(sqlmock.Sqlmock){
				func(mock sqlmock.Sqlmock) {
					rows := sqlmock.NewRows([]string{"id", "name"}).
						AddRow(5, "test")
					mock.ExpectQuery("^SELECT \\* FROM example_models WHERE id = \\$1").WillReturnRows(rows)
				},
			},
			model:    &ExampleModel{},
			id:       5,
			expected: &ExampleModel{ID: 5, Name: "test"},
			err:      nil,
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
			mapper := &Mapper{
				Conn:   sqlx.NewDb(db, "sqlmock"), // "sqlmock" is a magic string @ sqlmock for driver name
				logger: logrus.New(),
			}
			err = mapper.Find(tt.model, tt.id)
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Error(err)
			}
			if tt.expected != nil && !reflect.DeepEqual(tt.model, tt.expected) {
				t.Errorf("results don't match. Expected: %+v\nReceived: %+v", tt.expected, tt.model)
			}
			t.Logf("Returned: %+v", tt.model)
		})
	}
}
