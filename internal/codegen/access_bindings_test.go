package codegen

import (
	"reflect"
	"testing"
)

func TestAccessBindingMarshalPreservesPresence(t *testing.T) {
	maps := []struct {
		name  string
		value map[string]string
	}{
		{"omitted", nil},
		{"empty", map[string]string{}},
		{"mapped", map[string]string{"owner": "customer_id"}},
	}
	for _, rows := range maps {
		for _, writes := range maps {
			t.Run(rows.name+"/"+writes.name, func(t *testing.T) {
				binding := AccessBindingSpec{Rows: rows.value, Writes: writes.value}
				value, err := binding.MarshalYAML()
				if err != nil {
					t.Fatal(err)
				}
				mappings, ok := value.(map[string]map[string]string)
				if !ok {
					t.Fatalf("unexpected YAML mapping type %T", value)
				}
				for name, original := range map[string]map[string]string{"rows": rows.value, "writes": writes.value} {
					actual, present := mappings[name]
					if present != (original != nil) || !reflect.DeepEqual(actual, original) {
						t.Fatalf("%s: presence/value changed: present=%v, got=%#v, original=%#v", name, present, actual, original)
					}
				}
				if !reflect.DeepEqual(binding, AccessBindingSpec{Rows: rows.value, Writes: writes.value}) {
					t.Fatal("marshaling changed the original binding")
				}
			})
		}
	}
}
