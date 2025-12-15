package dbx

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/lib/pq"
)

type RowMapper struct {
	cache sync.Map
}

func NewRowMapper() *RowMapper {
	return &RowMapper{}
}

func (m *RowMapper) Map(dest any, columns []string, scanner func(dest ...any) error) error {
	t := reflect.TypeOf(dest)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var typeMapper TypeMapper

	if cached, ok := m.cache.Load(t); ok {
		typeMapper = cached.(TypeMapper)
	} else {
		typeMapper = NewTypeMapper(t)
		m.cache.Store(t, typeMapper)
	}

	pointers := make([]any, len(columns))
	for i, c := range columns {
		mapping := typeMapper.Fields[c]
		field := reflect.ValueOf(dest).Elem()

		for _, f := range mapping.FieldName {
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					field.Set(reflect.New(field.Type().Elem()))
				}
				field = field.Elem()
			}
			field = field.FieldByName(f)
		}

		if !field.CanAddr() {
			panic(fmt.Sprintf("Field not found for column %s", c))
		}

		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() != reflect.Uint8 {
			obj := reflect.New(reflect.MakeSlice(field.Type(), 0, 0).Type()).Elem()
			field.Set(obj)
			pointers[i] = pq.Array(field.Addr().Interface())
		} else {
			pointers[i] = field.Addr().Interface()
		}
	}

	return scanner(pointers...)
}

type TypeMapper struct {
	Type   reflect.Type
	Fields map[string]FieldInfo
}

func NewTypeMapper(t reflect.Type) TypeMapper {
	all := make(map[string]FieldInfo)

	if t.Kind() != reflect.Struct {
		return TypeMapper{
			Type:   t,
			Fields: all,
		}
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		columnName := field.Tag.Get("db")
		if columnName != "" {
			fieldType := field.Type
			fieldKind := fieldType.Kind()

			if fieldKind == reflect.Ptr {
				fieldType = field.Type.Elem()
				mapper := NewTypeMapper(fieldType)
				for _, f := range mapper.Fields {
					all[columnName+"_"+f.ColumnName] = FieldInfo{
						ColumnName: columnName + "_" + f.ColumnName,
						FieldName:  append([]string{field.Name}, f.FieldName...),
					}
				}
			} else {
				all[columnName] = FieldInfo{
					FieldName:  []string{field.Name},
					ColumnName: columnName,
				}
			}
		}
	}
	return TypeMapper{
		Type:   t,
		Fields: all,
	}
}

type FieldInfo struct {
	FieldName  []string
	ColumnName string
}
