package environment

import (
	"bytes"
	"fmt"
	"strings"
)

// StructObject represents an instantiated struct object containing properties.
type StructObject struct {
	StructName string
	Fields     map[string]Object
}

// Type returns the ObjectType of the struct instance.
func (s *StructObject) Type() ObjectType {
	return ObjectType(s.StructName)
}

// Inspect returns a string representation of the struct object.
func (s *StructObject) Inspect() string {
	var out bytes.Buffer

	out.WriteString(s.StructName)
	out.WriteString(" { ")

	var fields []string
	for name, val := range s.Fields {
		fields = append(fields, fmt.Sprintf("%s: %s", name, val.Inspect()))
	}

	out.WriteString(strings.Join(fields, ", "))
	out.WriteString(" }")

	return out.String()
}
