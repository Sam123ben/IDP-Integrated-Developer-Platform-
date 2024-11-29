// backend/utils/filesystem.go

package utils

import (
	"backend/models"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// LoadConfig reads the configuration from a JSON file
func LoadConfig(path string) (*models.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config models.Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Convert a value to a JSON string
func toJSON(value interface{}) (string, error) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// CreateDirectories ensures that the specified directories exist
func CreateDirectories(paths []string) error {
	for _, path := range paths {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

// WriteFile writes content to a specified path
func WriteFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0644)
}

// FormatDefault formats the default value of a variable
func formatDefault(varDef models.Variable) string {
	switch varDef.Type {
	case "bool", "number":
		return fmt.Sprintf("%v", varDef.Default)
	case "string":
		// Check if default is an expression
		if expr, ok := varDef.Default.(string); ok && strings.HasPrefix(expr, "var.") {
			return expr // Expression
		}
		return fmt.Sprintf("\"%v\"", varDef.Default)
	case "list(string)", "set(string)":
		list, ok := varDef.Default.([]interface{})
		if !ok {
			return "[]"
		}
		var items []string
		for _, item := range list {
			items = append(items, fmt.Sprintf("\"%v\"", item))
		}
		if varDef.Type == "set(string)" {
			return fmt.Sprintf("toset([%s])", strings.Join(items, ", "))
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case "map(string)":
		var entries []string
		switch v := varDef.Default.(type) {
		case map[string]interface{}:
			for key, val := range v {
				entries = append(entries, fmt.Sprintf("\"%s\" = \"%v\"", key, val))
			}
		case map[string]string:
			for key, val := range v {
				entries = append(entries, fmt.Sprintf("\"%s\" = \"%s\"", key, val))
			}
		}
		return fmt.Sprintf("{ %s }", strings.Join(entries, ", "))
	case "object({ provision_vm_agent = bool, enable_automatic_upgrades = bool })",
		"object({ publisher = string, offer = string, sku = string, version = string })",
		"object({ name = string, caching = string, create_option = string, managed_disk_type = string })":
		// Assume default is a map[string]interface{}
		objMap, ok := varDef.Default.(map[string]interface{})
		if !ok {
			return "{}"
		}
		var items []string
		for key, val := range objMap {
			switch v := val.(type) {
			case string:
				items = append(items, fmt.Sprintf("\"%s\" = \"%v\"", key, v))
			default:
				items = append(items, fmt.Sprintf("\"%s\" = %v", key, v))
			}
		}
		return fmt.Sprintf("{ %s }", strings.Join(items, ", "))
	case "tuple":
		tuple, ok := varDef.Default.([]interface{})
		if !ok {
			return "[]"
		}
		var items []string
		for _, item := range tuple {
			switch v := item.(type) {
			case string:
				items = append(items, fmt.Sprintf("\"%v\"", v))
			default:
				items = append(items, fmt.Sprintf("%v", v))
			}
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	default:
		return fmt.Sprintf("%v", varDef.Default)
	}
}

// GenerateFileFromTemplate generates a file from a template
func GenerateFileFromTemplate(templatePath, destinationPath string, data interface{}) error {
	funcMap := template.FuncMap{
		"title": cases.Title(language.Und).String,
		"add":   func(a, b int) int { return a + b },
		"toJSON": func(value interface{}) string {
			jsonString, err := toJSON(value)
			if err != nil {
				return "null"
			}
			return jsonString
		},
		"typeOf": func(value interface{}) string {
			switch value.(type) {
			case string:
				return "string"
			case bool:
				return "bool"
			case int, float64:
				return "number"
			case []interface{}:
				return "list"
			case map[string]interface{}:
				return "map"
			case map[string]string:
				return "map(string)"
			default:
				return "any"
			}
		},
		"or": func(a, b interface{}) interface{} {
			if a != nil {
				return a
			}
			return b
		},
		"formatValue": func(value interface{}, varType string) string {
			switch varType {
			case "bool", "number":
				return fmt.Sprintf("%v", value)
			case "string":
				// Determine if value is an expression or a literal
				expr, ok := value.(string)
				if ok && strings.HasPrefix(expr, "var.") {
					return expr // Expression
				}
				return fmt.Sprintf("\"%v\"", value) // Literal
			case "list(string)", "set(string)":
				list, ok := value.([]interface{})
				if !ok {
					return "[]"
				}
				var items []string
				for _, item := range list {
					items = append(items, fmt.Sprintf("\"%v\"", item))
				}
				if varType == "set(string)" {
					return fmt.Sprintf("toset([%s])", strings.Join(items, ", "))
				}
				return fmt.Sprintf("[%s]", strings.Join(items, ", "))
			case "map(string)":
				var entries []string
				switch v := value.(type) {
				case map[string]interface{}:
					for key, val := range v {
						entries = append(entries, fmt.Sprintf("\"%s\" = \"%v\"", key, val))
					}
				case map[string]string:
					for key, val := range v {
						entries = append(entries, fmt.Sprintf("\"%s\" = \"%s\"", key, val))
					}
				}
				return fmt.Sprintf("{ %s }", strings.Join(entries, ", "))
			case "object({ provision_vm_agent = bool, enable_automatic_upgrades = bool })",
				"object({ publisher = string, offer = string, sku = string, version = string })",
				"object({ name = string, caching = string, create_option = string, managed_disk_type = string })":
				// Assume value is an expression like var.os_profile_windows_config
				expr, ok := value.(string)
				if ok {
					return expr
				}
				return "{}" // Default to empty object if not an expression
			case "tuple":
				tuple, ok := value.([]interface{})
				if !ok {
					return "[]"
				}
				var items []string
				for _, item := range tuple {
					switch item.(type) {
					case string:
						items = append(items, fmt.Sprintf("\"%v\"", item))
					default:
						items = append(items, fmt.Sprintf("%v", item))
					}
				}
				return fmt.Sprintf("[%s]", strings.Join(items, ", "))
			default:
				return fmt.Sprintf("%v", value)
			}
		},
		// backend/utils/filesystem.go

		"formatDefault": func(varDef models.Variable) string {
			switch varDef.Type {
			case "bool", "number":
				return fmt.Sprintf("%v", varDef.Default)
			case "string":
				// Check if default is an expression
				if expr, ok := varDef.Default.(string); ok && strings.HasPrefix(expr, "var.") {
					return expr // Expression
				}
				return fmt.Sprintf("\"%v\"", varDef.Default)
			case "list(string)", "set(string)":
				list, ok := varDef.Default.([]interface{})
				if !ok {
					return "[]"
				}
				var items []string
				for _, item := range list {
					items = append(items, fmt.Sprintf("\"%v\"", item))
				}
				if varDef.Type == "set(string)" {
					return fmt.Sprintf("toset([%s])", strings.Join(items, ", "))
				}
				return fmt.Sprintf("[%s]", strings.Join(items, ", "))
			case "map(string)":
				var entries []string
				switch v := varDef.Default.(type) {
				case map[string]interface{}:
					for key, val := range v {
						entries = append(entries, fmt.Sprintf("\"%s\" = \"%v\"", key, val))
					}
				case map[string]string:
					for key, val := range v {
						entries = append(entries, fmt.Sprintf("\"%s\" = \"%s\"", key, val))
					}
				}
				return fmt.Sprintf("{ %s }", strings.Join(entries, ", "))
			case "object({ provision_vm_agent = bool, enable_automatic_upgrades = bool })",
				"object({ publisher = string, offer = string, sku = string, version = string })",
				"object({ name = string, caching = string, create_option = string, managed_disk_type = string })":
				// Assume default is a map[string]interface{}
				objMap, ok := varDef.Default.(map[string]interface{})
				if !ok {
					return "{}"
				}
				var items []string
				for key, val := range objMap {
					switch v := val.(type) {
					case string:
						items = append(items, fmt.Sprintf("\"%s\" = \"%v\"", key, v))
					default:
						items = append(items, fmt.Sprintf("\"%s\" = %v", key, v))
					}
				}
				return fmt.Sprintf("{ %s }", strings.Join(items, ", "))
			case "tuple":
				tuple, ok := varDef.Default.([]interface{})
				if !ok {
					return "[]"
				}
				var items []string
				for _, item := range tuple {
					switch v := item.(type) {
					case string:
						items = append(items, fmt.Sprintf("\"%v\"", v))
					default:
						items = append(items, fmt.Sprintf("%v", v))
					}
				}
				return fmt.Sprintf("[%s]", strings.Join(items, ", "))
			default:
				return fmt.Sprintf("%v", varDef.Default)
			}
		},
	}

	// Parse the template with the function map
	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		return err
	}

	// Ensure the destination directory exists
	destDir := filepath.Dir(destinationPath)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return err
	}

	// Execute the template
	var outputBuffer bytes.Buffer
	if err := tmpl.Execute(&outputBuffer, data); err != nil {
		return err
	}

	return WriteFile(destinationPath, outputBuffer.Bytes())
}
