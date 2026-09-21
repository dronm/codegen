package codegen

// AccessBindingSpec preserves the distinction between omitted mappings and an
// explicitly empty map. Read-only representations must not declare Writes;
// writable representations may deliberately declare writes: {}.
type AccessBindingSpec struct {
	Rows   map[string]string `yaml:"rows"`
	Writes map[string]string `yaml:"writes"`
}

// MarshalYAML omits nil maps without omitting explicitly empty maps. Plain YAML
// map encoding turns nil into {}, while omitempty drops both; either loses the
// presence information used by access validation. Decoding remains the ordinary
// strict struct decoder, including its unknown-field checks.
func (binding AccessBindingSpec) MarshalYAML() (any, error) {
	mappings := make(map[string]map[string]string, 2)
	if binding.Rows != nil {
		mappings["rows"] = binding.Rows
	}
	if binding.Writes != nil {
		mappings["writes"] = binding.Writes
	}
	return mappings, nil
}
