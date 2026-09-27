package dbx

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/lib/pq"
)

type RowMapper struct {
	types sync.Map
}

type mappedField struct {
	indices  []int
	indirect bool
	array    bool
}

type rowMapping struct {
	fields  []mappedField
	targets []any
}

func NewRowMapper() *RowMapper {
	return &RowMapper{}
}

func (mapper *RowMapper) Map(destination any, columns []string, scan func(...any) error) error {
	row := reflect.ValueOf(destination).Elem()
	mapping, err := mapper.prepare(row.Type(), columns)
	if err != nil {
		return err
	}

	return mapping.scan(row, scan)
}

func (mapper *RowMapper) prepare(rowType reflect.Type, columns []string) (rowMapping, error) {
	if rowType.Kind() != reflect.Struct {
		if len(columns) != 1 {
			return rowMapping{}, fmt.Errorf("cannot map %d columns to non-struct type %s", len(columns), rowType.Name())
		}

		return rowMapping{targets: make([]any, 1)}, nil
	}

	var fields map[string]mappedField
	if cached, ok := mapper.types.Load(rowType); ok {
		fields = cached.(map[string]mappedField)
	} else {
		fields = make(map[string]mappedField)
		mapFields(rowType, "", nil, false, fields)
		cached, _ := mapper.types.LoadOrStore(rowType, fields)
		fields = cached.(map[string]mappedField)
	}

	mapping := rowMapping{
		fields:  make([]mappedField, len(columns)),
		targets: make([]any, len(columns)),
	}

	for i, column := range columns {
		field, exists := fields[column]
		if !exists {
			panic(fmt.Sprintf("Column %s not found in type %s", column, rowType.Name()))
		}

		mapping.fields[i] = field
	}

	return mapping, nil
}

func mapFields(rowType reflect.Type, prefix string, parent []int, indirect bool, fields map[string]mappedField) {
	if rowType.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < rowType.NumField(); i++ {
		field := rowType.Field(i)
		name := field.Tag.Get("db")
		if name == "" {
			continue
		}

		indices := make([]int, len(parent)+1)
		copy(indices, parent)
		indices[len(parent)] = i
		column := prefix + name
		if field.Type.Kind() == reflect.Ptr {
			mapFields(field.Type.Elem(), column+"_", indices, true, fields)
			continue
		}

		fields[column] = mappedField{
			indices:  indices,
			indirect: indirect,
			array:    field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() != reflect.Uint8,
		}
	}
}

// Scan targets belong to one result set and are reused only after Scan returns.
func (mapping *rowMapping) scan(row reflect.Value, scan func(...any) error) error {
	if mapping.fields == nil {
		mapping.targets[0] = row.Addr().Interface()
		return scan(mapping.targets...)
	}

	for i, column := range mapping.fields {
		field := row
		for _, index := range column.indices {
			field = field.Field(index)
			if column.indirect && field.Kind() == reflect.Ptr {
				if field.IsNil() {
					field.Set(reflect.New(field.Type().Elem()))
				}
				field = field.Elem()
			}
		}

		if column.array {
			mapping.targets[i] = pq.Array(field.Addr().Interface())
		} else {
			mapping.targets[i] = field.Addr().Interface()
		}
	}

	return scan(mapping.targets...)
}
