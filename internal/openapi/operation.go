package openapi

// Operation is a normalized OpenAPI operation used by Discovery and Tool generation.
type Operation struct {
	APIName     string
	OperationID string
	Method      string
	Path        string
	Tags        []string
	Summary     string
	Description string
	Parameters  []Parameter
	// RequestBody nil when absent or unsupported media type (caller may skip tool).
	RequestBody *RequestBody
	SkipReason  string // non-empty => do not generate tool (e.g. unsupported body)
}

// Parameter is an OpenAPI parameter (path/query/header).
type Parameter struct {
	Name        string
	In          string // path | query | header
	Required    bool
	Description string
	Schema      map[string]interface{}
}

// RequestBody describes JSON request body projection.
type RequestBody struct {
	Required bool
	// ExpandObject: when true, Properties are merged into tool input root.
	ExpandObject bool
	Properties   map[string]interface{}
	RequiredProps []string
	// BodySchema used when ExpandObject is false (non-object schema → single "body" param).
	BodySchema map[string]interface{}
}

// Matcher matches an Operation against discovery rules.
type Matcher interface {
	Match(op *Operation) bool
}
