package dmpr

import "testing"

func TestEQ_Where(t *testing.T) {
	tests := []struct {
		name string
		col  string
		val  interface{}
		want string
	}{
		{
			name: "nil value",
			col:  "one",
			want: "one IS NULL",
		},
		{
			name: "scalar value",
			col:  "one",
			val:  "two",
			want: "one = :one",
		},
		{
			name: "slice value",
			col:  "one",
			val:  []string{"two"},
			want: "one IN (:one)",
		},
		{
			name: "empty slice",
			col:  "one",
			val:  []string{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := Eq(tt.col, tt.val)
			if got := op.Where(); got != tt.want {
				t.Errorf("EQ.Where() = %v, want %v", got, tt.want)
			}
		})
	}
}
