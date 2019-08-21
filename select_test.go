package dmpr_test

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/julian7/dmpr"
	"github.com/julian7/tester"
	"github.com/pkg/errors"
	"gopkg.in/guregu/null.v3"
)

type ExampleBelongsTo struct {
	ID     int
	Name   string
	Extras null.String      `db:"extras,omitempty"`
	OneID  int              `db:"one_id"`
	MoreID int              `db:"more_id"`
	One    ExampleHasOne    `db:"one,belongs"`
	More   []ExampleHasMany `db:"many,belongs"`
}

type ExampleHasOne struct {
	ID      int
	Name    string
	Belongs *ExampleBelongsTo `db:"belongs,relation=one"`
}

type ExampleHasMany struct {
	ID      int
	Name    string
	Belongs []*ExampleBelongsTo `db:"belongs,relation=many"`
}

func TestSelectQuery_All(t *testing.T) {
	tests := []struct {
		name     string
		model    interface{}
		prep     func(*dmpr.SelectQuery)
		mock     func(sqlmock.Sqlmock)
		expected interface{}
		err      error
	}{
		{
			name:  "nil model",
			model: nil,
			err:   errors.New("Invalid Model Type"),
		},
		{
			name:  "unexported model",
			model: &[]struct{}{},
			err:   errors.New("Invalid Model Type"),
		},
		{
			name:  "get all fields",
			model: &[]ExampleBelongsTo{},
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "one_id", "more_id"}).
					AddRow(1, "test", 2, 3)
				mock.ExpectQuery(`^SELECT t1\.id, t1\.name, t1\.extras, t1\.one_id, t1\.more_id FROM example_belongs_toes t1`).WillReturnRows(rows)
			},
			expected: &[]ExampleBelongsTo{
				{ID: 1, Name: "test", OneID: 2, MoreID: 3},
			},
		},
		{
			name:  "get selected fields",
			model: &[]ExampleBelongsTo{},
			prep:  func(s *dmpr.SelectQuery) { s.Select("id", "extras") },
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "extras"}).
					AddRow(1, "test")
				mock.ExpectQuery(`^SELECT id, extras FROM example_belongs_toes t1`).WillReturnRows(rows)
			},
			expected: &[]ExampleBelongsTo{
				{ID: 1, Extras: null.StringFrom("test")},
			},
		},
		{
			name:  "filter query",
			model: &[]ExampleBelongsTo{},
			prep:  func(s *dmpr.SelectQuery) { s.Where(dmpr.Eq("id", 3)); s.Where(dmpr.Eq("extras", nil)) },
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "extras", "one_id", "more_id"}).
					AddRow(3, "test", nil, 0, 0)
				mock.ExpectQuery(fmt.Sprintf("^%s", regexp.QuoteMeta(
					`SELECT t1.id, t1.name, t1.extras, t1.one_id, t1.more_id `+
						`FROM example_belongs_toes t1 WHERE id = :id AND extras IS NULL`,
				))).WillReturnRows(rows)
			},
			expected: &[]ExampleBelongsTo{
				{ID: 3, Name: "test", Extras: null.String{}, OneID: 0, MoreID: 0},
			},
		},
		{
			name:  "belongs to",
			model: &[]ExampleBelongsTo{},
			prep:  func(s *dmpr.SelectQuery) { s.Join("one") },
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "extras", "one_id", "more_id", "one_id", "one_name"}).
					AddRow(1, "test", nil, 2, 0, 2, "subname")
				mock.ExpectQuery(fmt.Sprintf("^%s", regexp.QuoteMeta(
					`SELECT t1.id, t1.name, t1.extras, t1.one_id, t1.more_id, t2.id AS one_id, t2.name AS one_name `+
						`FROM example_belongs_toes t1 LEFT JOIN example_has_ones t2 ON (t1.one_id=t2.id)`))).WillReturnRows(rows)
			},
			expected: &[]ExampleBelongsTo{
				{ID: 1, Name: "test", Extras: null.String{}, OneID: 2, MoreID: 0, One: ExampleHasOne{
					ID: 2, Name: "subname",
				}},
			},
		},
		{
			name:  "has one",
			model: &[]ExampleHasOne{},
			prep:  func(s *dmpr.SelectQuery) { s.Join("belongs") },
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "belongs_id", "belongs_name", "belongs_extras", "belongs_one_id", "belongs_more_id"}).
					AddRow(1, "test", 2, "subname", nil, 0, 1)
				mock.ExpectQuery(fmt.Sprintf("^%s", regexp.QuoteMeta(
					`SELECT t1.id, t1.name, t2.id AS belongs_id, t2.name AS belongs_name, `+
						`t2.extras AS belongs_extras, t2.one_id AS belongs_one_id, t2.more_id AS belongs_more_id `+
						`FROM example_has_ones t1 LEFT JOIN example_belongs_toes t2 ON (t1.id=t2.one_id)`,
				))).WillReturnRows(rows)
			},
			expected: &[]ExampleHasOne{
				{
					ID:   1,
					Name: "test",
					Belongs: &ExampleBelongsTo{
						ID:     2,
						Name:   "subname",
						OneID:  0,
						MoreID: 1,
					},
				},
			},
		},
		{
			name:  "has many",
			model: &[]ExampleHasMany{},
			prep:  func(s *dmpr.SelectQuery) { s.Join("belongs") },
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "belongs_id", "belongs_name", "belongs_extras", "belongs_one_id", "belongs_more_id"}).
					AddRow(1, "test", 2, "subname", nil, 0, 1).
					AddRow(1, "test", 3, "subname2", nil, 0, 1)
				mock.ExpectQuery(fmt.Sprintf("^%s", regexp.QuoteMeta(
					`SELECT t1.id, t1.name, t2.id AS belongs_id, t2.name AS belongs_name, `+
						`t2.extras AS belongs_extras, t2.one_id AS belongs_one_id, t2.more_id AS belongs_more_id `+
						`FROM example_has_manies t1 LEFT JOIN example_belongs_toes t2 ON (t1.id=t2.many_id)`,
				))).WillReturnRows(rows)
			},
			expected: &[]ExampleHasMany{
				{
					ID:   1,
					Name: "test",
					Belongs: []*ExampleBelongsTo{
						{
							ID:     2,
							Name:   "subname",
							OneID:  0,
							MoreID: 1,
						},
						{
							ID:     3,
							Name:   "subname2",
							OneID:  0,
							MoreID: 1,
						},
					},
				},
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
			mapper := dmpr.New("")
			mapper.Conn = sqlx.NewDb(db, "sqlmock") // "sqlmock" is a magic string @ sqlmock for driver name
			q, err := mapper.NewSelect(tt.model)
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}

			if tt.prep != nil {
				tt.prep(q)
			}
			if tt.mock != nil {
				tt.mock(mock)
			}
			err = q.All()
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}
			if tt.mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Error(err)
				}
			}
			if tt.expected != nil && !reflect.DeepEqual(tt.model, tt.expected) {
				t.Errorf("results don't match. Expected: %+v\nReceived: %+v", tt.expected, tt.model)
			}
		})
	}
}
