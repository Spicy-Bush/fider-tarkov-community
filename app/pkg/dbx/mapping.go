package dbx

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/lib/pq"
)

type RowMapper struct {
	cache sync.Map
}

func NewRowMapper() *RowMapper {
	return &RowMapper{}
}

// columnKey creates a cache key from column names
// uses \x00 as separator since it can't appear in column names
func columnKey(columns []string) string {
	return strings.Join(columns, "\x00")
}

// cachedMapping holds pre-computed field indices for a type+columns combination
type cachedMapping struct {
	fieldIndices [][]int // indices for each column
	isSlice      []bool  // whether field is a slice (needs pq.Array)
	hasPtr       []bool  // whether field path has a pointer that might be nil
}

func (m *RowMapper) Map(dest any, columns []string, scanner func(dest ...any) error) error {
	destVal := reflect.ValueOf(dest).Elem()
	t := destVal.Type()

	// gotta handle non struct types (e.g., int, string) so we'll just scan directly into dest for now
	if t.Kind() != reflect.Struct {
		if len(columns) == 1 {
			return scanner(dest)
		}
		return fmt.Errorf("cannot map %d columns to non-struct type %s", len(columns), t.Name())
	}

	// try to get cached mapping for this type+columns combo
	// use \x00 separator since it can't appear in type names or column names.. hopefully??????
	cacheKey := t.String() + "\x00" + columnKey(columns)

	var mapping *cachedMapping
	if cached, ok := m.cache.Load(cacheKey); ok {
		mapping = cached.(*cachedMapping)
	} else {
		// Build mapping
		mapping = m.buildMapping(t, columns)
		m.cache.Store(cacheKey, mapping)
	}

	// build pointers slice using cached indices
	pointers := make([]any, len(columns))
	for i := range columns {
		field := destVal

		indices := mapping.fieldIndices[i]
		if mapping.hasPtr[i] {
			// slow path need to check/init nil pointers along the way
			for _, idx := range indices {
				field = field.Field(idx)
				if field.Kind() == reflect.Ptr {
					if field.IsNil() {
						field.Set(reflect.New(field.Type().Elem()))
					}
					field = field.Elem()
				}
			}
		} else {
			// zoomie path, direct field access, no pointers yipeee
			for _, idx := range indices {
				field = field.Field(idx)
			}
		}

		if mapping.isSlice[i] {
			// reset slice to empty
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
			pointers[i] = pq.Array(field.Addr().Interface())
		} else {
			pointers[i] = field.Addr().Interface()
		}
	}

	return scanner(pointers...)
}

func (m *RowMapper) buildMapping(t reflect.Type, columns []string) *cachedMapping {
	// first get the TypeMapper for field name lookups
	var typeMapper TypeMapper
	if cached, ok := m.cache.Load(t); ok {
		typeMapper = cached.(TypeMapper)
	} else {
		typeMapper = NewTypeMapper(t)
		m.cache.Store(t, typeMapper)
	}

	mapping := &cachedMapping{
		fieldIndices: make([][]int, len(columns)),
		isSlice:      make([]bool, len(columns)),
		hasPtr:       make([]bool, len(columns)),
	}

	for i, c := range columns {
		fieldInfo, exists := typeMapper.Fields[c]
		if !exists {
			panic(fmt.Sprintf("Column %s not found in type %s", c, t.Name()))
		}

		indices := make([]int, 0, len(fieldInfo.FieldName))
		hasPtr := false

		currentType := t
		for _, fname := range fieldInfo.FieldName {
			if currentType.Kind() == reflect.Ptr {
				currentType = currentType.Elem()
				hasPtr = true
			}
			f, ok := currentType.FieldByName(fname)
			if !ok {
				panic(fmt.Sprintf("Field %s not found in type %s for column %s", fname, t.Name(), c))
			}
			indices = append(indices, f.Index[0])
			currentType = f.Type
		}

		mapping.fieldIndices[i] = indices
		mapping.hasPtr[i] = hasPtr

		// check if it's a slice type (but not []byte)
		finalType := currentType
		if finalType.Kind() == reflect.Ptr {
			finalType = finalType.Elem()
		}
		mapping.isSlice[i] = finalType.Kind() == reflect.Slice && finalType.Elem().Kind() != reflect.Uint8
	}

	return mapping
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
