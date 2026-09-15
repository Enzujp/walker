package walker

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// JSONExample builds a synthetic example from a Go type, respecting encoding/json
// field visibility, tags, embedded fields, and custom marshalers. It never uses
// values from the supplied instance. Recursive references terminate as null.
// Examples are illustrative, not schema validation or realistic test data.
func JSONExample(value any) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	v, err := sample(reflect.TypeOf(value), make(map[reflect.Type]bool), 0)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(v.Interface(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal example: %w", err)
	}
	return data, nil
}

func sample(t reflect.Type, active map[reflect.Type]bool, depth int) (reflect.Value, error) {
	v := reflect.New(t).Elem()
	if active[t] || depth >= 32 {
		return v, nil
	}
	active[t] = true
	defer delete(active, t)
	if t == reflect.TypeOf(time.Time{}) {
		return reflect.ValueOf(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)), nil
	}
	switch t.Kind() {
	case reflect.Pointer:
		child, err := sample(t.Elem(), active, depth+1)
		if err != nil {
			return v, err
		}
		v.Set(reflect.New(t.Elem()))
		v.Elem().Set(child)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" || field.Tag.Get("json") == "-" {
				continue
			}
			child, err := sample(field.Type, active, depth+1)
			if err != nil {
				return v, fmt.Errorf("field %s: %w", field.Name, err)
			}
			v.Field(i).Set(child)
		}
	case reflect.String:
		v.SetString("string")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Slice:
		child, err := sample(t.Elem(), active, depth+1)
		if err != nil {
			return v, err
		}
		v.Set(reflect.MakeSlice(t, 1, 1))
		v.Index(0).Set(child)
	case reflect.Array:
		for i := 0; i < t.Len(); i++ {
			child, err := sample(t.Elem(), active, depth+1)
			if err != nil {
				return v, err
			}
			v.Index(i).Set(child)
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(t))
	case reflect.Interface:
		// No concrete type can be inferred from an interface.
	default:
		return v, fmt.Errorf("unsupported example type: %s", t)
	}
	return v, nil
}
