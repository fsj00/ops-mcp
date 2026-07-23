package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type rawDoc struct {
	OpenAPI    string                     `yaml:"openapi" json:"openapi"`
	Paths      map[string]rawPathItem     `yaml:"paths" json:"paths"`
	Components rawComponents              `yaml:"components" json:"components"`
}

type rawComponents struct {
	Schemas map[string]interface{} `yaml:"schemas" json:"schemas"`
}

type rawPathItem struct {
	Parameters []rawParameter `yaml:"parameters" json:"parameters"`
	Get        *rawOperation  `yaml:"get" json:"get"`
	Put        *rawOperation  `yaml:"put" json:"put"`
	Post       *rawOperation  `yaml:"post" json:"post"`
	Delete     *rawOperation  `yaml:"delete" json:"delete"`
	Options    *rawOperation  `yaml:"options" json:"options"`
	Head       *rawOperation  `yaml:"head" json:"head"`
	Patch      *rawOperation  `yaml:"patch" json:"patch"`
	Trace      *rawOperation  `yaml:"trace" json:"trace"`
}

type rawOperation struct {
	OperationID string                 `yaml:"operationId" json:"operationId"`
	Tags        []string               `yaml:"tags" json:"tags"`
	Summary     string                 `yaml:"summary" json:"summary"`
	Description string                 `yaml:"description" json:"description"`
	Parameters  []rawParameter         `yaml:"parameters" json:"parameters"`
	RequestBody *rawRequestBody        `yaml:"requestBody" json:"requestBody"`
}

type rawParameter struct {
	Name        string                 `yaml:"name" json:"name"`
	In          string                 `yaml:"in" json:"in"`
	Required    bool                   `yaml:"required" json:"required"`
	Description string                 `yaml:"description" json:"description"`
	Schema      map[string]interface{} `yaml:"schema" json:"schema"`
	Ref         string                 `yaml:"$ref" json:"$ref"`
}

type rawRequestBody struct {
	Required bool                              `yaml:"required" json:"required"`
	Content  map[string]rawMediaType           `yaml:"content" json:"content"`
	Ref      string                            `yaml:"$ref" json:"$ref"`
}

type rawMediaType struct {
	Schema map[string]interface{} `yaml:"schema" json:"schema"`
}

var httpMethods = []struct {
	name string
	get  func(*rawPathItem) *rawOperation
}{
	{"GET", func(p *rawPathItem) *rawOperation { return p.Get }},
	{"PUT", func(p *rawPathItem) *rawOperation { return p.Put }},
	{"POST", func(p *rawPathItem) *rawOperation { return p.Post }},
	{"DELETE", func(p *rawPathItem) *rawOperation { return p.Delete }},
	{"OPTIONS", func(p *rawPathItem) *rawOperation { return p.Options }},
	{"HEAD", func(p *rawPathItem) *rawOperation { return p.Head }},
	{"PATCH", func(p *rawPathItem) *rawOperation { return p.Patch }},
	{"TRACE", func(p *rawPathItem) *rawOperation { return p.Trace }},
}

func loadDoc(path string) (*rawDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read openapi %s: %w", path, err)
	}
	var doc rawDoc
	// Try YAML first (also accepts JSON).
	if err := yaml.Unmarshal(data, &doc); err != nil {
		if err2 := json.Unmarshal(data, &doc); err2 != nil {
			return nil, fmt.Errorf("parse openapi %s: %v", path, err)
		}
	}
	if doc.OpenAPI == "" {
		return nil, fmt.Errorf("openapi %s: missing openapi version field", path)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil, fmt.Errorf("openapi %s: unsupported version %q (need 3.x)", path, doc.OpenAPI)
	}
	return &doc, nil
}

func extractOperations(apiName string, doc *rawDoc) ([]*Operation, []string) {
	var ops []*Operation
	var warns []string
	seenIDs := map[string]string{} // operationId -> path+method

	for path, item := range doc.Paths {
		pathParams := item.Parameters
		for _, m := range httpMethods {
			raw := m.get(&item)
			if raw == nil {
				continue
			}
			if raw.OperationID == "" {
				warns = append(warns, fmt.Sprintf("%s %s: missing operationId, skipped", m.name, path))
				continue
			}
			if prev, ok := seenIDs[raw.OperationID]; ok {
				warns = append(warns, fmt.Sprintf("duplicate operationId %q (%s and %s %s)", raw.OperationID, prev, m.name, path))
				// Mark as hard error via special skip — caller treats duplicate as failure.
				ops = append(ops, &Operation{
					APIName:     apiName,
					OperationID: raw.OperationID,
					Method:      m.name,
					Path:        path,
					SkipReason:  "duplicate_operation_id",
				})
				continue
			}
			seenIDs[raw.OperationID] = m.name + " " + path

			op := &Operation{
				APIName:     apiName,
				OperationID: raw.OperationID,
				Method:      m.name,
				Path:        path,
				Tags:        append([]string{}, raw.Tags...),
				Summary:     raw.Summary,
				Description: raw.Description,
			}
			params, w := mergeParameters(pathParams, raw.Parameters)
			warns = append(warns, w...)
			op.Parameters = params

			if raw.RequestBody != nil {
				rb, skip, w := projectRequestBody(raw.RequestBody, doc)
				warns = append(warns, w...)
				if skip != "" {
					op.SkipReason = skip
				} else {
					op.RequestBody = rb
				}
			}
			ops = append(ops, op)
		}
	}
	return ops, warns
}

func mergeParameters(pathLevel, opLevel []rawParameter) ([]Parameter, []string) {
	var warns []string
	byKey := map[string]Parameter{}
	order := []string{}
	add := func(rp rawParameter) {
		if rp.Ref != "" {
			warns = append(warns, fmt.Sprintf("parameter $ref %q not supported in MVP, skipped", rp.Ref))
			return
		}
		in := strings.ToLower(rp.In)
		if in == "cookie" {
			return
		}
		if in != "path" && in != "query" && in != "header" {
			return
		}
		key := in + ":" + rp.Name
		schema := rp.Schema
		if schema == nil {
			schema = map[string]interface{}{"type": "string"}
		}
		if _, exists := byKey[key]; !exists {
			order = append(order, key)
		}
		byKey[key] = Parameter{
			Name:        rp.Name,
			In:          in,
			Required:    rp.Required || in == "path",
			Description: rp.Description,
			Schema:      schema,
		}
	}
	for _, p := range pathLevel {
		add(p)
	}
	for _, p := range opLevel {
		add(p)
	}
	out := make([]Parameter, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out, warns
}

func projectRequestBody(rb *rawRequestBody, doc *rawDoc) (*RequestBody, string, []string) {
	var warns []string
	if rb.Ref != "" {
		return nil, "requestBody $ref not supported", []string{fmt.Sprintf("requestBody $ref %q skipped", rb.Ref)}
	}
	if len(rb.Content) == 0 {
		return nil, "", nil
	}
	mt, schema, ok := pickJSONMedia(rb.Content)
	if !ok {
		return nil, "unsupported requestBody media type", []string{"requestBody has no application/json (or compatible) content, skipped"}
	}
	schema = resolveSchema(schema, doc, &warns)
	if schema == nil {
		return nil, "unresolvable requestBody schema", warns
	}
	typ, _ := schema["type"].(string)
	if typ == "object" || (typ == "" && schema["properties"] != nil) {
		props, _ := asStringMap(schema["properties"])
		if props == nil {
			props = map[string]interface{}{}
		}
		var req []string
		if r, ok := schema["required"].([]interface{}); ok {
			for _, v := range r {
				if s, ok := v.(string); ok {
					req = append(req, s)
				}
			}
		}
		_ = mt
		return &RequestBody{
			Required:      rb.Required,
			ExpandObject:  true,
			Properties:    props,
			RequiredProps: req,
		}, "", warns
	}
	return &RequestBody{
		Required:     rb.Required,
		ExpandObject: false,
		BodySchema:   schema,
	}, "", warns
}

func pickJSONMedia(content map[string]rawMediaType) (string, map[string]interface{}, bool) {
	if mt, ok := content["application/json"]; ok && mt.Schema != nil {
		return "application/json", mt.Schema, true
	}
	if mt, ok := content["*/*"]; ok && mt.Schema != nil {
		return "*/*", mt.Schema, true
	}
	for k, mt := range content {
		if strings.Contains(k, "json") && mt.Schema != nil {
			return k, mt.Schema, true
		}
	}
	return "", nil, false
}

func resolveSchema(schema map[string]interface{}, doc *rawDoc, warns *[]string) map[string]interface{} {
	if schema == nil {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok && ref != "" {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		if name == ref || doc.Components.Schemas == nil {
			*warns = append(*warns, fmt.Sprintf("unsupported schema $ref %q", ref))
			return nil
		}
		raw, ok := doc.Components.Schemas[name]
		if !ok {
			*warns = append(*warns, fmt.Sprintf("schema $ref %q not found", ref))
			return nil
		}
		resolved, ok := asStringMap(raw)
		if !ok {
			*warns = append(*warns, fmt.Sprintf("schema %q is not an object", name))
			return nil
		}
		return resolved
	}
	return schema
}

func asStringMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[ks] = val
		}
		return out, true
	default:
		return nil, false
	}
}
