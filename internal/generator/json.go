package generator

import (
	"encoding/json"
	"reflect"
)

func GenerateJSONExample(v interface{}) ([]byte, error) {
	result := build(reflect.TypeOf(v))

	return json.MarshalIndent(result, "", " ")
}

// Recursive struct parser
func build(t reflect.Type) interface{} {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		obj := make(map[string]interface{})
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			jsonTag := field.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}

			obj[jsonTag] = build(field.Type)
		}
		return obj

	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int64, reflect.Int32:
		return 0
	case reflect.Bool:
		return false
	case reflect.Float64, reflect.Float32:
		return 0.0
	case reflect.Slice:
		return []interface{}{build(t.Elem())}
	default:
		return nil
	}
}