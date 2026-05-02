package specs

import (
	"reflect"
	"testing"
	"time"
)

type schemaWidget struct {
	ID        string            `json:"id" required:"true"`
	Count     int               `json:"count"`
	Active    bool              `json:"active"`
	Tags      []string          `json:"tags"`
	Fixed     [2]int            `json:"fixed"`
	Metadata  map[string]string `json:"metadata"`
	Nested    schemaNested      `json:"nested"`
	Optional  *schemaNested     `json:"optional"`
	CreatedAt time.Time         `json:"created_at"`
	Ignored   string            `json:"-"`
}

type schemaNested struct {
	Name string `json:"name" binding:"required"`
}

type recursiveSchema struct {
	Next *recursiveSchema `json:"next"`
}

func TestSchemaFromStruct(t *testing.T) {
	schema, err := SchemaFrom[schemaWidget](SchemaOptions{})
	if err != nil {
		t.Fatalf("SchemaFrom() error = %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("type = %v, want object", schema["type"])
	}
	props := asMap(t, schema["properties"])
	if _, ok := props["Ignored"]; ok {
		t.Fatal("ignored field should not be present")
	}
	assertSchemaType(t, props, "id", "string")
	assertSchemaType(t, props, "count", "integer")
	assertSchemaType(t, props, "active", "boolean")
	assertSchemaType(t, props, "tags", "array")
	assertSchemaType(t, props, "metadata", "object")
	createdAt := asMap(t, props["created_at"])
	if createdAt["type"] != "string" || createdAt["format"] != "date-time" {
		t.Fatalf("created_at = %#v", createdAt)
	}
	optional := asMap(t, props["optional"])
	if optional["nullable"] != true {
		t.Fatalf("optional = %#v, want nullable", optional)
	}
	fixed := asMap(t, props["fixed"])
	if fixed["minItems"] != 2 || fixed["maxItems"] != 2 {
		t.Fatalf("fixed = %#v", fixed)
	}
	required := schemaStringSlice(t, schema["required"])
	if !reflect.DeepEqual(required, []string{"id"}) {
		t.Fatalf("required = %#v", required)
	}
	nested := asMap(t, props["nested"])
	nestedRequired := schemaStringSlice(t, nested["required"])
	if !reflect.DeepEqual(nestedRequired, []string{"name"}) {
		t.Fatalf("nested required = %#v", nestedRequired)
	}
}

func TestSchemaFromTypeRejectsUnsupportedTypes(t *testing.T) {
	if _, err := SchemaFromType(reflect.TypeOf(map[int]string{}), SchemaOptions{}); err == nil {
		t.Fatal("expected non-string map key error")
	}
	if _, err := SchemaFrom[recursiveSchema](SchemaOptions{}); err == nil {
		t.Fatal("expected recursive type error")
	}
}

func TestRegisterSchemaFrom(t *testing.T) {
	registry := NewRegistry(Info{Title: "Schemas", Version: "1"})
	if err := RegisterSchemaFrom[schemaNested](registry, "Nested", SchemaOptions{}); err != nil {
		t.Fatalf("RegisterSchemaFrom() error = %v", err)
	}
	doc := decodeOpenAPI(t, registry)
	components := asMap(t, doc["components"])
	schemas := asMap(t, components["schemas"])
	if _, ok := schemas["Nested"]; !ok {
		t.Fatalf("schemas = %#v, want Nested", schemas)
	}
}

func assertSchemaType(t *testing.T, props map[string]any, name, want string) {
	t.Helper()
	schema := asMap(t, props[name])
	if schema["type"] != want {
		t.Fatalf("%s type = %#v, want %s", name, schema, want)
	}
}

func schemaStringSlice(t *testing.T, value any) []string {
	t.Helper()
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				t.Fatalf("value has non-string item: %#v", item)
			}
			out = append(out, text)
		}
		return out
	default:
		t.Fatalf("value has type %T, want []string or []any: %#v", value, value)
		return nil
	}
}
