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

type schemaTaggedFields struct {
	State    string `json:"state" enum:"draft,active,archived" example:"active" required:"true"`
	Attempts int    `json:"attempts" enum:"1,2,3" example:"2"`
	Retried  bool   `json:"retried" example:"true"`
	Deleted  string `json:"deleted" nullable:"true"`
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

func TestSchemaFromStructTagsAddsEnumsExamplesAndNullable(t *testing.T) {
	schema, err := SchemaFrom[schemaTaggedFields](SchemaOptions{})
	if err != nil {
		t.Fatalf("SchemaFrom() error = %v", err)
	}
	props := asMap(t, schema["properties"])
	state := asMap(t, props["state"])
	if enum := schemaStringSlice(t, state["enum"]); !reflect.DeepEqual(enum, []string{"draft", "active", "archived"}) {
		t.Fatalf("state enum = %#v", enum)
	}
	if state["example"] != "active" {
		t.Fatalf("state example = %#v", state["example"])
	}
	attempts := asMap(t, props["attempts"])
	if enum := attempts["enum"]; !reflect.DeepEqual(enum, []any{int64(1), int64(2), int64(3)}) {
		t.Fatalf("attempts enum = %#v", enum)
	}
	if attempts["example"] != int64(2) {
		t.Fatalf("attempts example = %#v", attempts["example"])
	}
	retried := asMap(t, props["retried"])
	if retried["example"] != true {
		t.Fatalf("retried example = %#v", retried["example"])
	}
	deleted := asMap(t, props["deleted"])
	if deleted["nullable"] != true {
		t.Fatalf("deleted = %#v, want nullable", deleted)
	}
}

func TestSchemaHelperFunctionsDoNotMutateInput(t *testing.T) {
	base := map[string]any{"type": "string"}

	nullable := NullableSchema(base)
	if nullable["nullable"] != true || base["nullable"] != nil {
		t.Fatalf("NullableSchema nullable = %#v base = %#v", nullable, base)
	}
	withExample := SchemaWithExample(base, "active")
	if withExample["example"] != "active" || base["example"] != nil {
		t.Fatalf("SchemaWithExample schema = %#v base = %#v", withExample, base)
	}
	withEnum := SchemaWithEnum(base, "draft", "active")
	if enum := schemaStringSlice(t, withEnum["enum"]); !reflect.DeepEqual(enum, []string{"draft", "active"}) {
		t.Fatalf("SchemaWithEnum enum = %#v", enum)
	}
	if base["enum"] != nil {
		t.Fatalf("SchemaWithEnum mutated base = %#v", base)
	}
	ref := SchemaRef("#/components/schemas/Widget")
	if ref["$ref"] != "#/components/schemas/Widget" {
		t.Fatalf("SchemaRef = %#v", ref)
	}
}

func TestSchemaFromTypeRejectsUnsupportedTypes(t *testing.T) {
	if _, err := SchemaFromType(reflect.TypeOf(map[int]string{}), SchemaOptions{}); err == nil {
		t.Fatal("expected non-string map key error")
	}
	if _, err := SchemaFrom[recursiveSchema](SchemaOptions{}); err == nil {
		t.Fatal("expected recursive type error")
	}
	type invalidExample struct {
		Count int `json:"count" example:"not-an-int"`
	}
	if _, err := SchemaFrom[invalidExample](SchemaOptions{}); err == nil {
		t.Fatal("expected invalid example tag error")
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
