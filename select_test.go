package dmpr

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/julian7/tester"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"
)

type AllSelectorExample struct {
	ID     int
	Name   string
	Extras null.String           `db:"extras,omitempty"`
	SelID  int                   `db:"sel_id"`
	SubID  int                   `db:"sub_id"`
	Sub    AllSelectorSubExample `db:"sub,belongs"`
}

type AllSelectorSubExample struct {
	ID   int
	Name string
	Sel  *AllSelectorExample `db:"sel,relation=sel"`
}

type AllSelectorManyExample struct {
	ID   int
	Name string
	Sel  *[]AllSelectorManySubExample `db:"sel,relation=sel"`
}

type AllSelectorManySubExample struct {
	ID   int
	Name string
	Sel  *[]AllSelectorManyExample `db:"sel,relation=sel"`
}

func TestSelectQuery_allSelector(t *testing.T) {
	tests := []struct {
		name  string
		model interface{}
		prep  []func(*SelectQuery)
		want  string
		maps  []interface{}
		err   error
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
			model: &[]AllSelectorExample{},
			want:  "SELECT t1.id, t1.name, t1.extras, t1.sel_id, t1.sub_id FROM all_selector_examples t1",
		},
		{
			name:  "get selected fields",
			model: &[]AllSelectorExample{},
			prep: []func(*SelectQuery){
				func(s *SelectQuery) { s.Select("id", "extras") },
			},
			want: "SELECT id, extras FROM all_selector_examples t1",
		},
		{
			name:  "filter query",
			model: &[]AllSelectorExample{},
			prep: []func(*SelectQuery){
				func(s *SelectQuery) { s.Where(Eq("id", 3)) },
				func(s *SelectQuery) { s.Where(Eq("extras", nil)) },
			},
			maps: []interface{}{3, nil},
			want: "SELECT t1.id, t1.name, t1.extras, t1.sel_id, t1.sub_id FROM all_selector_examples t1 WHERE id = :id AND extras IS NULL",
		},
		{
			name:  "belongs to",
			model: &[]AllSelectorExample{},
			prep: []func(*SelectQuery){
				func(s *SelectQuery) { s.Join("sub") },
			},
			want: "SELECT t1.id, t1.name, t1.extras, t1.sel_id, t1.sub_id, t2.id AS sub_id, t2.name AS sub_name " +
				"FROM all_selector_examples t1 LEFT JOIN all_selector_sub_examples t2 ON (t1.sub_id=t2.id)",
		},
		{
			name:  "has one",
			model: &[]AllSelectorSubExample{},
			prep: []func(*SelectQuery){
				func(s *SelectQuery) { s.Join("sel") },
			},
			want: "SELECT t1.id, t1.name, t2.id AS sel_id, t2.name AS sel_name, t2.extras AS sel_extras, " +
				"t2.sel_id AS sel_sel_id, t2.sub_id AS sel_sub_id " +
				"FROM all_selector_sub_examples t1 LEFT JOIN all_selector_examples t2 ON (t1.id=t2.sel_id)",
		},
		{
			name:  "has many",
			model: &[]AllSelectorManyExample{},
			prep: []func(*SelectQuery){
				func(s *SelectQuery) { s.Join("sel") },
			},
			want: "SELECT t1.id, t1.name, t2.id AS sel_id, t2.name AS sel_name " +
				"FROM all_selector_many_examples t1 LEFT JOIN all_selector_many_sub_examples t2 ON (t1.id=t2.sel_id)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()
			mapper := &Mapper{
				Conn:   sqlx.NewDb(db, "sqlmock"), // "sqlmock" is a magic string @ sqlmock for driver name
				logger: logrus.New(),
			}
			q, err := mapper.NewSelect(tt.model)
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}

			for _, prepfunc := range tt.prep {
				prepfunc(q)
			}
			got, maps, err := q.allSelector()
			if assert := tester.AssertError(tt.err, err); assert != nil {
				t.Error(assert)
			}
			if err != nil {
				return
			}

			if got != tt.want {
				t.Errorf("SelectQuery.allSelector() got\n%v\nwant\n%v", got, tt.want)
			}
			if tt.maps == nil {
				tt.maps = []interface{}{}
			}
			if !reflect.DeepEqual(maps, tt.maps) {
				t.Errorf("SelectQuery.allSelector() maps = %v, want %v", maps, tt.maps)
			}

		})
	}
}
