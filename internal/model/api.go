package model

import "time"

// APIEndpoint is the upstream HTTP connection settings for one API service.
type APIEndpoint struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	Timeout   string `yaml:"timeout" json:"timeout"` // duration string, e.g. 10s
	VerifyTLS *bool  `yaml:"verify_tls" json:"verify_tls"`
}

// VerifyTLSEnabled returns whether TLS certificate verification is enabled (default true).
func (e APIEndpoint) VerifyTLSEnabled() bool {
	if e.VerifyTLS == nil {
		return true
	}
	return *e.VerifyTLS
}

// TimeoutDuration parses endpoint.timeout; returns 0 if empty/invalid (caller applies default).
func (e APIEndpoint) TimeoutDuration() time.Duration {
	if e.Timeout == "" {
		return 0
	}
	d, err := time.ParseDuration(e.Timeout)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

// APIOpenAPI locates the local OpenAPI document.
type APIOpenAPI struct {
	Path string `yaml:"path" json:"path"`
}

// DiscoveryRule is one include/exclude rule (fields AND; values within a field OR).
type DiscoveryRule struct {
	OperationIDs []string `yaml:"operation_ids" json:"operation_ids,omitempty"`
	Methods      []string `yaml:"methods" json:"methods,omitempty"`
	Paths        []string `yaml:"paths" json:"paths,omitempty"`
	Tags         []string `yaml:"tags" json:"tags,omitempty"`
}

// APIDiscovery controls which OpenAPI operations become MCP tools.
type APIDiscovery struct {
	Include []DiscoveryRule `yaml:"include" json:"include,omitempty"`
	Exclude []DiscoveryRule `yaml:"exclude" json:"exclude,omitempty"`
}

// APIService is one apis.yaml entry.
type APIService struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	OpenAPI     APIOpenAPI        `yaml:"openapi" json:"openapi"`
	Endpoint    APIEndpoint       `yaml:"endpoint" json:"endpoint"`
	Prefix      string            `yaml:"prefix" json:"prefix"`
	Labels      map[string]string `yaml:"labels" json:"labels,omitempty"`
	Headers     map[string]string `yaml:"headers" json:"-"`
	Discovery   APIDiscovery      `yaml:"discovery" json:"discovery"`
}

// APISummary is a safe view for API/Agent (no header values).
type APISummary struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels,omitempty"`
	BaseURL     string            `json:"base_url"`
	Prefix      string            `json:"prefix"`
	OpenAPIPath string            `json:"openapi_path"`
	Timeout     string            `json:"timeout,omitempty"`
	VerifyTLS   bool              `json:"verify_tls"`
	HasHeaders  bool              `json:"has_headers"`
	ToolCount   int               `json:"tool_count"`
}

// ToSummary strips header values. ToolCount is filled by the OpenAPI registry when listing.
func (a APIService) ToSummary() APISummary {
	return APISummary{
		Name:        a.Name,
		Description: a.Description,
		Labels:      a.Labels,
		BaseURL:     a.Endpoint.BaseURL,
		Prefix:      a.Prefix,
		OpenAPIPath: a.OpenAPI.Path,
		Timeout:     a.Endpoint.Timeout,
		VerifyTLS:   a.Endpoint.VerifyTLSEnabled(),
		HasHeaders:  len(a.Headers) > 0,
		ToolCount:   0,
	}
}

// APIsFile is the apis.yaml root.
type APIsFile struct {
	APIs []APIService `yaml:"apis"`
}
