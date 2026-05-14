package specs

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SchemaOptions configures schema generation from Go types.
type SchemaOptions struct {
	RefPrefix string
}

// SchemaFrom generates an OpenAPI schema from T.
func SchemaFrom[T any](opts SchemaOptions) (map[string]any, error) {
	var zero T
	return SchemaFromType(reflect.TypeOf(zero), opts)
}

// SchemaFromType generates an OpenAPI schema from a Go type.
func SchemaFromType(typ reflect.Type, opts SchemaOptions) (map[string]any, error) {
	if typ == nil {
		return nil, fmt.Errorf("schema type is required")
	}
	return schemaForType(indirectSchemaType(typ), opts, map[reflect.Type]bool{})
}

// RegisterSchemaFrom generates and registers an OpenAPI schema component.
func RegisterSchemaFrom[T any](registry *Registry, name string, opts SchemaOptions) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	schema, err := SchemaFrom[T](opts)
	if err != nil {
		return err
	}
	registry.RegisterSchema(name, schema)
	return nil
}

// SchemaRef returns a schema reference object.
func SchemaRef(ref string) map[string]any {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return map[string]any{}
	}
	return map[string]any{"$ref": ref}
}

// NullableSchema returns a copy of schema marked nullable.
func NullableSchema(schema map[string]any) map[string]any {
	out := cloneAnyMap(schema)
	out["nullable"] = true
	return out
}

// SchemaWithExample returns a copy of schema with an OpenAPI example value.
func SchemaWithExample(schema map[string]any, example any) map[string]any {
	out := cloneAnyMap(schema)
	out["example"] = example
	return out
}

// SchemaWithEnum returns a copy of schema with OpenAPI enum values.
func SchemaWithEnum(schema map[string]any, values ...any) map[string]any {
	out := cloneAnyMap(schema)
	if len(values) > 0 {
		out["enum"] = append([]any(nil), values...)
	}
	return out
}

func schemaForType(typ reflect.Type, opts SchemaOptions, stack map[reflect.Type]bool) (map[string]any, error) {
	if typ == nil {
		return nil, fmt.Errorf("schema type is required")
	}
	if typ == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}, nil
	}
	if stack[typ] {
		return nil, fmt.Errorf("recursive schema type %s is unsupported", typ)
	}
	if typ.Kind() == reflect.Pointer {
		schema, err := schemaForType(typ.Elem(), opts, stack)
		if err != nil {
			return nil, err
		}
		out := cloneAnyMap(schema)
		out["nullable"] = true
		return out, nil
	}
	switch typ.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return integerSchema(typ), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return integerSchema(typ), nil
	case reflect.Float32, reflect.Float64:
		return numberSchema(typ), nil
	case reflect.Slice, reflect.Array:
		itemSchema, err := schemaForType(typ.Elem(), opts, stack)
		if err != nil {
			return nil, err
		}
		out := map[string]any{"type": "array", "items": itemSchema}
		if typ.Kind() == reflect.Array {
			out["minItems"] = typ.Len()
			out["maxItems"] = typ.Len()
		}
		return out, nil
	case reflect.Map:
		if typ.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map schema keys must be strings")
		}
		valueSchema, err := schemaForType(typ.Elem(), opts, stack)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "object", "additionalProperties": valueSchema}, nil
	case reflect.Struct:
		stack[typ] = true
		defer delete(stack, typ)
		return structSchema(typ, opts, stack)
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Pointer,
		reflect.UnsafePointer:
		return nil, fmt.Errorf("unsupported schema type %s", typ)
	}
	return nil, fmt.Errorf("unsupported schema type %s", typ)
}

func structSchema(typ reflect.Type, opts SchemaOptions, stack map[reflect.Type]bool) (map[string]any, error) {
	properties := map[string]any{}
	var required []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := schemaFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		fieldSchema, err := schemaForType(field.Type, opts, stack)
		if err != nil {
			return nil, err
		}
		fieldSchema, err = applyFieldSchemaTags(field, fieldSchema)
		if err != nil {
			return nil, err
		}
		properties[name] = fieldSchema
		if requiredSchemaField(field) {
			required = append(required, name)
		}
	}
	out := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		sort.Strings(required)
		out["required"] = required
	}
	return out, nil
}

func applyFieldSchemaTags(field reflect.StructField, schema map[string]any) (map[string]any, error) {
	out := cloneAnyMap(schema)
	if isTrueTag(field.Tag.Get("nullable")) {
		out["nullable"] = true
	}
	if raw := strings.TrimSpace(field.Tag.Get("example")); raw != "" {
		value, err := parseSchemaLiteral(raw, field.Type)
		if err != nil {
			return nil, fmt.Errorf("%s example tag: %w", field.Name, err)
		}
		out["example"] = value
	}
	if raw := strings.TrimSpace(field.Tag.Get("enum")); raw != "" {
		parts := strings.Split(raw, ",")
		values := make([]any, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			value, err := parseSchemaLiteral(part, field.Type)
			if err != nil {
				return nil, fmt.Errorf("%s enum tag: %w", field.Name, err)
			}
			values = append(values, value)
		}
		if len(values) > 0 {
			out["enum"] = values
		}
	}
	return out, nil
}

func isTrueTag(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "true" || value == "1" || value == "yes"
}

func parseSchemaLiteral(raw string, typ reflect.Type) (any, error) {
	typ = indirectSchemaType(typ)
	if typ == reflect.TypeOf(time.Time{}) {
		return raw, nil
	}
	switch typ.Kind() {
	case reflect.String:
		return raw, nil
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, err
		}
		return value, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		return raw, nil
	}
	return raw, nil
}

func schemaFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "-"
	}
	if tag != "" {
		name := strings.TrimSpace(strings.Split(tag, ",")[0])
		if name != "" {
			return name
		}
	}
	return field.Name
}

func requiredSchemaField(field reflect.StructField) bool {
	return field.Tag.Get("required") == "true" || strings.Contains(field.Tag.Get("binding"), "required")
}

func integerSchema(typ reflect.Type) map[string]any {
	out := map[string]any{"type": "integer"}
	kind := typ.Kind()
	if kind == reflect.Int32 || kind == reflect.Uint32 {
		out["format"] = "int32"
	}
	if kind == reflect.Int || kind == reflect.Int64 || kind == reflect.Uint || kind == reflect.Uint64 {
		out["format"] = "int64"
	}
	return out
}

func numberSchema(typ reflect.Type) map[string]any {
	out := map[string]any{"type": "number"}
	if typ.Kind() == reflect.Float32 {
		out["format"] = "float"
	} else {
		out["format"] = "double"
	}
	return out
}

func indirectSchemaType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}
