package dbx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type mappedUser struct {
	ID   sql.NullInt64  `db:"id"`
	Name sql.NullString `db:"name"`
}

type mappedRow struct {
	ID      int         `db:"id"`
	User    *mappedUser `db:"user"`
	Tags    []string    `db:"tags"`
	Payload []byte      `db:"payload"`
}

type mappingDriver struct{}
type mappingConnection struct{}
type mappingTransaction struct{}
type mappingRows struct {
	mode  string
	index int
}

func (mappingDriver) Open(string) (driver.Conn, error) {
	return mappingConnection{}, nil
}

func (mappingConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (mappingConnection) Close() error { return nil }

func (mappingConnection) Begin() (driver.Tx, error) {
	return mappingTransaction{}, nil
}

func (mappingTransaction) Commit() error   { return nil }
func (mappingTransaction) Rollback() error { return nil }

func (mappingConnection) QueryContext(_ context.Context, mode string, _ []driver.NamedValue) (driver.Rows, error) {
	if mode == "query failure" {
		return nil, errors.New(mode)
	}

	return &mappingRows{mode: mode}, nil
}

func (*mappingRows) Columns() []string {
	return []string{"id", "user_id", "user_name", "tags", "payload"}
}

func (*mappingRows) Close() error { return nil }

func (rows *mappingRows) Next(values []driver.Value) error {
	if rows.mode == "empty" || rows.index == 3 {
		return io.EOF
	}
	if rows.mode == "read failure" && rows.index == 1 {
		return errors.New(rows.mode)
	}

	values[0] = int64(rows.index + 1)
	values[1] = int64(7)
	values[2] = "Author"
	values[3] = `{"first","second"}`
	values[4] = []byte("payload")
	if rows.index == 1 {
		values[1], values[2], values[3] = nil, nil, nil
	} else if rows.index == 2 {
		values[3] = "{}"
	}
	if rows.mode == "scan failure" && rows.index == 1 {
		values[0] = "invalid integer"
	}

	rows.index++
	return nil
}

func init() {
	sql.Register("mapping-test", mappingDriver{})
}

func TestSelectPreparedRows(t *testing.T) {
	database, err := sql.Open("mapping-test", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { database.Close() })

	for _, mode := range []string{"rows", "empty", "query failure", "read failure", "scan failure"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}

			defer tx.Rollback()
			trx := &Trx{tx: tx, ctx: context.Background()}
			existing := &mappedRow{ID: 99}
			rows := []*mappedRow{existing}

			err = trx.Select(&rows, mode)
			if strings.HasSuffix(mode, "failure") {
				if err == nil {
					t.Fatal("selection discarded a query, scan or row error")
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if mode != "rows" {
				if len(rows) != 1 || rows[0] != existing {
					t.Fatal("empty or failed selection replaced the caller's output")
				}

				return
			}

			if len(rows) != 3 || rows[0].ID != 1 || rows[1].ID != 2 || rows[2].ID != 3 {
				t.Fatalf("incorrect result identities: %+v", rows)
			}
			if rows[0].User.ID.Int64 != 7 || !rows[0].User.ID.Valid || rows[0].User.Name.String != "Author" {
				t.Fatalf("incorrect nested user: %+v", rows[0].User)
			}
			if rows[1].User == nil || rows[1].User.ID.Valid || rows[1].User.Name.Valid {
				t.Fatalf("SQL NULL nested values were lost: %+v", rows[1].User)
			}
			if !reflect.DeepEqual(rows[0].Tags, []string{"first", "second"}) || rows[1].Tags != nil || rows[2].Tags == nil || len(rows[2].Tags) != 0 {
				t.Fatal("nonempty, NULL and empty SQL arrays must remain distinct")
			}

			rows[0].User.Name.String = "changed"
			rows[0].Payload[0] = 'x'
			if rows[2].User.Name.String != "Author" || string(rows[2].Payload) != "payload" {
				t.Fatal("separate result rows share mutable storage")
			}
		})
	}
}

func TestPreparedMappingConcurrentColumnOrders(t *testing.T) {
	mapper := NewRowMapper()
	var workers sync.WaitGroup

	for worker := range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()

			columns := []string{"id", "user_name"}
			if worker%2 == 1 {
				columns = []string{"user_name", "id"}
			}

			mapping, err := mapper.prepare(reflect.TypeOf(mappedRow{}), columns)
			if err != nil {
				t.Error(err)
				return
			}

			for id := 1; id <= 20; id++ {
				row := &mappedRow{}
				err := mapping.scan(reflect.ValueOf(row).Elem(), func(targets ...any) error {
					for i, column := range columns {
						if column == "id" {
							*targets[i].(*int) = id
						} else {
							if err := targets[i].(*sql.NullString).Scan("Author"); err != nil {
								return err
							}
						}
					}

					return nil
				})
				if err != nil || row.ID != id || row.User.Name.String != "Author" {
					t.Errorf("incorrect concurrent row: %+v, %v", row, err)
					return
				}
			}
		}()
	}

	workers.Wait()

	types := 0
	mapper.types.Range(func(_, _ any) bool {
		types++
		return true
	})

	if types != 1 {
		t.Fatalf("column orders created %d retained type mappings", types)
	}
}

func TestRowMapperReplacesReusedArrays(t *testing.T) {
	type arrays struct {
		Tags    []string        `db:"tags"`
		Numbers []sql.NullInt64 `db:"numbers"`
	}

	mapper := NewRowMapper()
	row := arrays{Tags: []string{"old"}, Numbers: []sql.NullInt64{{Int64: 99, Valid: true}}}

	for _, test := range []struct {
		name    string
		tags    any
		numbers any
		want    arrays
	}{
		{
			name:    "populated",
			tags:    `{"first","second"}`,
			numbers: `{NULL,7}`,
			want: arrays{
				Tags:    []string{"first", "second"},
				Numbers: []sql.NullInt64{{}, {Int64: 7, Valid: true}},
			},
		},
		{
			name:    "empty",
			tags:    "{}",
			numbers: "{}",
			want:    arrays{Tags: []string{}, Numbers: []sql.NullInt64{}},
		},
		{name: "NULL"},
		{
			name:    "repopulated",
			tags:    `{"last"}`,
			numbers: `{9}`,
			want: arrays{
				Tags:    []string{"last"},
				Numbers: []sql.NullInt64{{Int64: 9, Valid: true}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := mapper.Map(&row, []string{"tags", "numbers"}, func(targets ...any) error {
				if err := targets[0].(sql.Scanner).Scan(test.tags); err != nil {
					return err
				}

				return targets[1].(sql.Scanner).Scan(test.numbers)
			})
			if err != nil || !reflect.DeepEqual(row, test.want) {
				t.Fatalf("array replacement: got %+v, want %+v, error %v", row, test.want, err)
			}
		})
	}
}

func TestRowMapperErrors(t *testing.T) {
	mapper := NewRowMapper()
	var scalar int
	err := mapper.Map(&scalar, []string{"first", "second"}, func(...any) error {
		t.Fatal("mismatched scalar columns reached Scan")
		return nil
	})
	if err == nil || err.Error() != "cannot map 2 columns to non-struct type int" {
		t.Fatalf("unexpected shape error: %v", err)
	}

	scanError := errors.New("scanner failed")
	if err := mapper.Map(&scalar, []string{"id"}, func(...any) error { return scanError }); err != scanError {
		t.Fatalf("scanner error was replaced: %v", err)
	}

	defer func() {
		if recovered := recover(); recovered != "Column absent not found in type mappedRow" {
			t.Errorf("unexpected missing-column diagnostic: %v", recovered)
		}
	}()

	mapper.Map(&mappedRow{}, []string{"absent"}, func(...any) error { return nil })
}
