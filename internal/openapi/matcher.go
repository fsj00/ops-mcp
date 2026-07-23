package openapi

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fsj00/ops-mcp/internal/model"
)

type alwaysMatcher struct{}

func (alwaysMatcher) Match(*Operation) bool { return true }

type neverMatcher struct{}

func (neverMatcher) Match(*Operation) bool { return false }

// OperationIDMatcher matches operationId against any of the regex patterns.
type OperationIDMatcher struct {
	res []*regexp.Regexp
}

func NewOperationIDMatcher(patterns []string) (*OperationIDMatcher, error) {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("operation_ids pattern %q: %w", p, err)
		}
		res = append(res, re)
	}
	return &OperationIDMatcher{res: res}, nil
}

func (m *OperationIDMatcher) Match(op *Operation) bool {
	for _, re := range m.res {
		if re.MatchString(op.OperationID) {
			return true
		}
	}
	return false
}

// MethodMatcher matches HTTP methods (case-insensitive).
type MethodMatcher struct {
	methods map[string]struct{}
}

func NewMethodMatcher(methods []string) *MethodMatcher {
	m := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		m[strings.ToUpper(method)] = struct{}{}
	}
	return &MethodMatcher{methods: m}
}

func (m *MethodMatcher) Match(op *Operation) bool {
	_, ok := m.methods[strings.ToUpper(op.Method)]
	return ok
}

// PathMatcher matches OpenAPI paths against discovery path patterns.
type PathMatcher struct {
	patterns []string
}

func NewPathMatcher(patterns []string) *PathMatcher {
	return &PathMatcher{patterns: patterns}
}

func (m *PathMatcher) Match(op *Operation) bool {
	for _, p := range m.patterns {
		ok, err := matchPath(p, op.Path)
		if err != nil {
			continue
		}
		if ok {
			return true
		}
	}
	return false
}

// ValidatePathPatterns compiles regex path patterns eagerly (load-time errors).
func ValidatePathPatterns(patterns []string) error {
	for _, p := range patterns {
		if strings.HasPrefix(p, "^") {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("paths pattern %q: %w", p, err)
			}
		}
	}
	return nil
}

// TagMatcher matches tags with exact string equality (OR).
type TagMatcher struct {
	tags map[string]struct{}
}

func NewTagMatcher(tags []string) *TagMatcher {
	m := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		m[t] = struct{}{}
	}
	return &TagMatcher{tags: m}
}

func (m *TagMatcher) Match(op *Operation) bool {
	for _, t := range op.Tags {
		if _, ok := m.tags[t]; ok {
			return true
		}
	}
	return false
}

// RuleMatcher ANDs the field matchers present on a DiscoveryRule.
type RuleMatcher struct {
	matchers []Matcher
}

func NewRuleMatcher(rule model.DiscoveryRule) (*RuleMatcher, error) {
	var ms []Matcher
	if len(rule.OperationIDs) > 0 {
		m, err := NewOperationIDMatcher(rule.OperationIDs)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	if len(rule.Methods) > 0 {
		ms = append(ms, NewMethodMatcher(rule.Methods))
	}
	if len(rule.Paths) > 0 {
		if err := ValidatePathPatterns(rule.Paths); err != nil {
			return nil, err
		}
		ms = append(ms, NewPathMatcher(rule.Paths))
	}
	if len(rule.Tags) > 0 {
		ms = append(ms, NewTagMatcher(rule.Tags))
	}
	// Empty rule => always match.
	if len(ms) == 0 {
		return &RuleMatcher{matchers: []Matcher{alwaysMatcher{}}}, nil
	}
	return &RuleMatcher{matchers: ms}, nil
}

func (m *RuleMatcher) Match(op *Operation) bool {
	for _, sub := range m.matchers {
		if !sub.Match(op) {
			return false
		}
	}
	return true
}

// orMatcher matches if any child matches; empty children => always (for include) or never (for exclude).
type orMatcher struct {
	children []Matcher
	empty    Matcher // used when children empty
}

func (m orMatcher) Match(op *Operation) bool {
	if len(m.children) == 0 {
		return m.empty.Match(op)
	}
	for _, c := range m.children {
		if c.Match(op) {
			return true
		}
	}
	return false
}

// DiscoveryMatcher implements Expose = Include AND NOT(Exclude).
type DiscoveryMatcher struct {
	include Matcher
	exclude Matcher
}

func NewDiscoveryMatcher(d model.APIDiscovery) (*DiscoveryMatcher, error) {
	incChildren := make([]Matcher, 0, len(d.Include))
	for i, rule := range d.Include {
		rm, err := NewRuleMatcher(rule)
		if err != nil {
			return nil, fmt.Errorf("discovery.include[%d]: %w", i, err)
		}
		incChildren = append(incChildren, rm)
	}
	excChildren := make([]Matcher, 0, len(d.Exclude))
	for i, rule := range d.Exclude {
		rm, err := NewRuleMatcher(rule)
		if err != nil {
			return nil, fmt.Errorf("discovery.exclude[%d]: %w", i, err)
		}
		excChildren = append(excChildren, rm)
	}
	return &DiscoveryMatcher{
		include: orMatcher{children: incChildren, empty: alwaysMatcher{}},
		exclude: orMatcher{children: excChildren, empty: neverMatcher{}},
	}, nil
}

func (d *DiscoveryMatcher) Match(op *Operation) bool {
	if !d.include.Match(op) {
		return false
	}
	if d.exclude.Match(op) {
		return false
	}
	return true
}
