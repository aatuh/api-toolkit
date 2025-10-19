package swagstub

// Spec is a minimal stub of github.com/swaggo/swag.Spec used by generated docs.
// This allows generated swagger documentation to work without importing the full swag package.
type Spec struct {
	Version          string
	Host             string
	BasePath         string
	Schemes          []string
	Title            string
	Description      string
	InfoInstanceName string
	SwaggerTemplate  string
	LeftDelim        string
	RightDelim       string
}

// InstanceName returns the registered instance name, defaulting to "swagger".
func (s *Spec) InstanceName() string {
	if s == nil {
		return "swagger"
	}
	if s.InfoInstanceName == "" {
		return "swagger"
	}
	return s.InfoInstanceName
}

var registry = make(map[string]*Spec)

// Register stores the spec under the provided name.
func Register(name string, spec *Spec) {
	if spec == nil {
		return
	}
	key := name
	if key == "" {
		key = spec.InstanceName()
	}
	registry[key] = spec
}

// Get retrieves a spec by name.
func Get(name string) *Spec {
	return registry[name]
}

// List returns all registered spec names.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
