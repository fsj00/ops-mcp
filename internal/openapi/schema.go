package openapi

import (
	"fmt"
	"strings"
)

// ToolMeta is the MCP-facing tool metadata generated from an Operation.
type ToolMeta struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	APIName     string
	Method      string
	Path        string
	// ParamIns maps tool argument name → OpenAPI "in" (path/query/header).
	ParamIns map[string]string
	// BodyMode: "" | "expand" | "body"
	BodyMode      string
	BodyRequired  bool
	BodyPropNames map[string]struct{} // when expand: which args come from body
}

func buildToolMeta(prefix string, op *Operation) (*ToolMeta, []string) {
	var warns []string
	desc := op.Summary
	if desc == "" {
		desc = op.Description
	}
	if desc == "" {
		desc = fmt.Sprintf("%s %s", op.Method, op.Path)
	}

	properties := map[string]interface{}{}
	required := []string{}
	paramIns := map[string]string{}

	for _, p := range op.Parameters {
		prop := map[string]interface{}{}
		for k, v := range p.Schema {
			prop[k] = v
		}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		if _, ok := prop["type"]; !ok {
			prop["type"] = "string"
		}
		properties[p.Name] = prop
		paramIns[p.Name] = p.In
		if p.Required {
			required = append(required, p.Name)
		}
	}

	bodyMode := ""
	bodyProps := map[string]struct{}{}
	if op.RequestBody != nil {
		if op.RequestBody.ExpandObject {
			bodyMode = "expand"
			for name, schema := range op.RequestBody.Properties {
				if _, exists := properties[name]; exists {
					warns = append(warns, fmt.Sprintf("%s: body property %q conflicts with parameter, skipped", op.OperationID, name))
					continue
				}
				prop, ok := asStringMap(schema)
				if !ok {
					prop = map[string]interface{}{"type": "string"}
				}
				properties[name] = prop
				bodyProps[name] = struct{}{}
			}
			for _, name := range op.RequestBody.RequiredProps {
				if _, ok := bodyProps[name]; ok {
					required = append(required, name)
				}
			}
			if op.RequestBody.Required && len(bodyProps) == 0 {
				// object with no usable props — still ok
			}
		} else {
			bodyMode = "body"
			schema := op.RequestBody.BodySchema
			if schema == nil {
				schema = map[string]interface{}{}
			}
			properties["body"] = schema
			paramIns["body"] = "body"
			if op.RequestBody.Required {
				required = append(required, "body")
			}
		}
	}

	// Dedupe required
	required = uniqueStrings(required)

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return &ToolMeta{
		Name:          prefix + op.OperationID,
		Description:   desc,
		InputSchema:   schema,
		APIName:       op.APIName,
		Method:        op.Method,
		Path:          op.Path,
		ParamIns:      paramIns,
		BodyMode:      bodyMode,
		BodyRequired:  op.RequestBody != nil && op.RequestBody.Required,
		BodyPropNames: bodyProps,
	}, warns
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func fillPath(template string, args map[string]interface{}) (string, error) {
	segs := strings.Split(template, "/")
	for i, seg := range segs {
		if isTemplateSeg(seg) {
			name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			v, ok := args[name]
			if !ok || v == nil {
				return "", fmt.Errorf("missing path parameter %q", name)
			}
			segs[i] = fmt.Sprint(v)
		}
	}
	return strings.Join(segs, "/"), nil
}
