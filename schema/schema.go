// Package schema holds the machine-readable Telegram Bot API schema for
// mtgo-bot-api. It describes every official Bot API method and the core object
// types, together with implementation status, so the same data can drive:
//
//   - implementation coverage checks (what is registered vs the spec),
//   - SDK/client generation for other languages,
//   - internal handler/struct generation and documentation.
//
// The data lives in JSON files next to this package (methods.json, types.json,
// status.json). Load() parses them relative to the package source directory.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Schema is the in-memory representation of the full Bot API schema loaded from
// the JSON data files.
type Schema struct {
	APIVersion string             `json:"api_version"` // official Bot API version, e.g. "10.1"
	Methods    []Method           `json:"methods"`
	Types      []TypeDef          `json:"types"`
	Status     map[string]Status  `json:"status"` // keyed by method or type name
}

// Method describes a single Bot API method.
type Method struct {
	// Name is the lowercased method name used on the wire, e.g. "sendmessage".
	// This matches the key in the official Client.cpp methods_ table and the
	// registration name used by internal/client.Register.
	Name string `json:"name"`
	// Title is the conventional camelCase form, e.g. "sendMessage".
	Title string `json:"title"`
	// Category groups related methods, e.g. "Messaging", "Stickers".
	Category string `json:"category"`
	// Description is a short human-readable summary of what the method does.
	Description string `json:"description"`
	// Returns is the Bot API return type, e.g. "Message", "Boolean", "User".
	// Array results use "[]" suffix, e.g. "Update[]", "BotCommand[]".
	Returns string `json:"returns"`
	// Parameters lists the method's input parameters in declaration order.
	Parameters []Parameter `json:"parameters"`
	// ParamsComplete is true when Parameters fully reflects the official Bot API
	// signature for this method. False means the parameter set is partial or not
	// yet authored (a known gap, not a parity failure).
	ParamsComplete bool `json:"params_complete"`
	// Official is true for methods defined by the official Telegram Bot API.
	// False marks mtgo-bot-api extensions (e.g. "close", "logout").
	Official bool `json:"official"`
}

// Parameter describes one input parameter of a Method.
type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "String", "Integer", "Float", "Boolean", a type name, or "Type[]"
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// TypeDef describes one Bot API object type (e.g. User, Chat, Message).
type TypeDef struct {
	Name        string `json:"name"` // "Message"
	Description string `json:"description"`
	Fields      []Field `json:"fields"`
}

// Field describes one field of a TypeDef.
type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// Status captures the implementation status of a method or type. It is an
// override layer: the default status is derived from introspection (a method is
// "implemented" if it is registered); status.json records extra nuance such as
// whether a method is a stub or has known gaps.
type Status struct {
	// State is one of: "implemented", "stub", "partial", "unsupported", "n/a".
	State string `json:"state"`
	// Note is an optional free-form explanation.
	Note string `json:"note,omitempty"`
}

// Load reads the schema JSON files from dir and returns the assembled Schema.
// dir should point at the schema/ package directory (the folder containing
// methods.json, types.json, status.json).
func Load(dir string) (*Schema, error) {
	var s Schema
	if err := readJSON(filepath.Join(dir, "methods.json"), &s); err != nil {
		return nil, fmt.Errorf("methods.json: %w", err)
	}
	if err := readJSON(filepath.Join(dir, "types.json"), &s); err != nil {
		return nil, fmt.Errorf("types.json: %w", err)
	}
	if err := readJSON(filepath.Join(dir, "status.json"), &s); err != nil {
		return nil, fmt.Errorf("status.json: %w", err)
	}
	if s.APIVersion == "" {
		return nil, fmt.Errorf("api_version is empty")
	}
	if s.Status == nil {
		s.Status = map[string]Status{}
	}
	return &s, nil
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// MethodByName returns the method with the given wire name, or nil.
func (s *Schema) MethodByName(name string) *Method {
	for i := range s.Methods {
		if s.Methods[i].Name == name {
			return &s.Methods[i]
		}
	}
	return nil
}

// MethodNames returns the wire names of all methods in the schema.
func (s *Schema) MethodNames() []string {
	out := make([]string, len(s.Methods))
	for i := range s.Methods {
		out[i] = s.Methods[i].Name
	}
	return out
}

// OfficialMethodNames returns the wire names of methods marked Official.
func (s *Schema) OfficialMethodNames() []string {
	var out []string
	for i := range s.Methods {
		if s.Methods[i].Official {
			out = append(out, s.Methods[i].Name)
		}
	}
	return out
}
