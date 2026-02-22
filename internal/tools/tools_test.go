package tools

import (
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestViewRangeSchemaNotNullable(t *testing.T) {
	schema, err := jsonschema.For[ViewArgs](&jsonschema.ForOptions{
		TypeSchemas: typeSchemas,
	})
	if err != nil {
		t.Fatal(err)
	}

	props := schema.Properties
	if props == nil {
		t.Fatal("expected properties in schema")
	}
	vrSchema, ok := props["view_range"]
	if !ok {
		t.Fatal("expected view_range in schema properties")
	}

	if vrSchema.Type != "array" {
		t.Errorf("view_range Type = %q, want %q", vrSchema.Type, "array")
	}
	if len(vrSchema.Types) != 0 {
		t.Errorf("view_range Types = %v, want empty (non-nullable)", vrSchema.Types)
	}
	if vrSchema.Items == nil || vrSchema.Items.Type != "integer" {
		t.Errorf("view_range Items should have Type \"integer\", got %+v", vrSchema.Items)
	}
}

func TestEditorViewRangeSchemaNotNullable(t *testing.T) {
	schema, err := jsonschema.For[StrReplaceEditorArgs](&jsonschema.ForOptions{
		TypeSchemas: typeSchemas,
	})
	if err != nil {
		t.Fatal(err)
	}

	props := schema.Properties
	if props == nil {
		t.Fatal("expected properties in schema")
	}
	vrSchema, ok := props["view_range"]
	if !ok {
		t.Fatal("expected view_range in schema properties")
	}

	if vrSchema.Type != "array" {
		t.Errorf("view_range Type = %q, want %q", vrSchema.Type, "array")
	}
	if len(vrSchema.Types) != 0 {
		t.Errorf("view_range Types = %v, want empty (non-nullable)", vrSchema.Types)
	}
	if vrSchema.Items == nil || vrSchema.Items.Type != "integer" {
		t.Errorf("view_range Items should have Type \"integer\", got %+v", vrSchema.Items)
	}
}

func TestTypeSchemasCoverage(t *testing.T) {
	// Ensure all expected types are in typeSchemas
	expected := []reflect.Type{
		reflect.TypeFor[EditorCommand](),
		reflect.TypeFor[ViewRange](),
	}
	for _, typ := range expected {
		if _, ok := typeSchemas[typ]; !ok {
			t.Errorf("typeSchemas missing entry for %v", typ)
		}
	}
}
