// Package spec implements a minimal, purpose-built parser for the
// specific (very regular, 2-space-indented, generator-produced) OpenAPI
// 3.0.3 YAML style used by ociregistry.yaml. It is NOT a general YAML or
// OpenAPI parser: it only understands the subset of structure needed to
// extract path templates, HTTP methods, and path/query parameters
// (name, in, required, type). If the spec file's formatting style changes
// substantially (flow-style mappings, anchors, multi-line scalars, tabs,
// different indent width, etc.) this parser will need to be adjusted.
package spec

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Param describes one OpenAPI parameter (path, query, or header).
type Param struct {
	Name     string
	In       string // "path", "query", "header"
	Required bool
	Type     string // "string", "integer", ... (defaults to "string")
}

// Operation describes one HTTP method on a path.
type Operation struct {
	Method      string // GET, PUT, POST, DELETE, PATCH, HEAD (uppercase)
	OperationID string
	Parameters  []Param
	Responses   []string // declared response status codes, e.g. "200","404"
}

// PathItem is one "paths:" entry: a template path plus its operations.
type PathItem struct {
	Path       string // e.g. "/v2/{s1}/{s2}/manifests/{reference}"
	Operations []Operation
}

// Spec is the parsed subset of the OpenAPI document that this tool needs.
type Spec struct {
	Paths []PathItem
}

// PathParams returns the path parameter names in the order they appear in
// the path template (which is what matters for substitution), regardless
// of the order they were declared in the "parameters:" list.
func (p PathItem) TemplateParamNames() []string {
	re := regexp.MustCompile(`\{([^{}]+)\}`)
	matches := re.FindAllStringSubmatch(p.Path, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names
}

// QueryParams returns only the query-string parameters of an operation.
func (o Operation) QueryParams() []Param {
	var out []Param
	for _, p := range o.Parameters {
		if p.In == "query" {
			out = append(out, p)
		}
	}
	return out
}

// Param looks up a declared parameter by name (path or query).
func (o Operation) Param(name string) (Param, bool) {
	for _, p := range o.Parameters {
		if p.Name == name {
			return p, true
		}
	}
	return Param{}, false
}

// ParseFile reads and parses an OpenAPI YAML file from disk.
func ParseFile(path string) (*Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening spec file: %w", err)
	}
	defer f.Close()
	return Parse(f)
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func indentOf(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

// Parse parses the (subset of) OpenAPI YAML needed by this tool from r.
func Parse(r io.Reader) (*Spec, error) {
	scanner := bufio.NewScanner(r)
	// Path/manifest/blob lines in generated specs can be long; be generous.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	sp := &Spec{}
	var (
		inPaths  bool
		curPath  *PathItem
		curOp    *Operation
		curParam *Param
		section  string // "", "parameters", "responses"
	)

	finalizeParam := func() {
		if curParam != nil && curOp != nil {
			curOp.Parameters = append(curOp.Parameters, *curParam)
		}
		curParam = nil
	}
	finalizeOp := func() {
		finalizeParam()
		if curOp != nil && curPath != nil {
			curPath.Operations = append(curPath.Operations, *curOp)
		}
		curOp = nil
	}
	finalizePath := func() {
		finalizeOp()
		if curPath != nil {
			sp.Paths = append(sp.Paths, *curPath)
		}
		curPath = nil
	}

	methodSet := map[string]bool{
		"get": true, "put": true, "post": true, "delete": true,
		"patch": true, "head": true, "options": true, "trace": true,
	}

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		trim := strings.TrimSpace(raw)
		if trim == "" {
			continue
		}
		indent := indentOf(raw)

		if indent == 0 {
			if trim == "paths:" {
				inPaths = true
			} else if inPaths {
				// Left the paths section (e.g. a sibling top-level key).
				finalizePath()
				inPaths = false
			}
			continue
		}
		if !inPaths {
			continue
		}

		switch {
		case indent == 2:
			finalizePath()
			key := unquote(strings.TrimSuffix(trim, ":"))
			curPath = &PathItem{Path: key}
			section = ""

		case indent == 4:
			key := strings.TrimSuffix(trim, ":")
			if !methodSet[strings.ToLower(key)] {
				// Not a method (shouldn't normally occur at this depth).
				continue
			}
			finalizeOp()
			curOp = &Operation{Method: strings.ToUpper(key)}
			section = ""

		case indent == 6:
			switch {
			case trim == "parameters:":
				section = "parameters"
			case trim == "parameters: []":
				finalizeParam()
				section = ""
			case strings.HasPrefix(trim, "- name:"):
				finalizeParam()
				name := unquote(strings.TrimSpace(strings.TrimPrefix(trim, "- name:")))
				curParam = &Param{Name: name, Type: "string"}
			case trim == "responses:":
				finalizeParam()
				section = "responses"
			case strings.HasPrefix(trim, "operationId:"):
				if curOp != nil {
					curOp.OperationID = strings.TrimSpace(strings.TrimPrefix(trim, "operationId:"))
				}
			default:
				// tags:, summary:, description: at operation level - ignored.
			}

		case indent == 8:
			switch section {
			case "parameters":
				if curParam == nil {
					continue
				}
				switch {
				case strings.HasPrefix(trim, "in:"):
					curParam.In = strings.TrimSpace(strings.TrimPrefix(trim, "in:"))
				case strings.HasPrefix(trim, "required:"):
					v := strings.TrimSpace(strings.TrimPrefix(trim, "required:"))
					curParam.Required = v == "true"
				case trim == "schema:":
					// type appears on the next (indent 10) line.
				default:
					// description:, etc - ignored.
				}
			case "responses":
				if strings.HasPrefix(trim, "'") && strings.HasSuffix(trim, "':") {
					code := strings.TrimSuffix(strings.TrimPrefix(trim, "'"), "':")
					if curOp != nil {
						curOp.Responses = append(curOp.Responses, code)
					}
				}
			}

		case indent == 10:
			if section == "parameters" && curParam != nil && strings.HasPrefix(trim, "type:") {
				curParam.Type = strings.TrimSpace(strings.TrimPrefix(trim, "type:"))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning spec: %w", err)
	}
	finalizePath()

	if len(sp.Paths) == 0 {
		return nil, fmt.Errorf("no paths found - is this a valid ociregistry-style OpenAPI 3.0.3 document?")
	}
	return sp, nil
}
