package openapi

import (
	"path"
	"regexp"
	"strings"
)

// matchPath reports whether openAPIPath matches a discovery path pattern.
func matchPath(pattern, openAPIPath string) (bool, error) {
	if strings.HasPrefix(pattern, "^") {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, err
		}
		return re.MatchString(openAPIPath), nil
	}
	if strings.ContainsAny(pattern, "*?") {
		return matchGlob(pattern, openAPIPath), nil
	}
	return matchTemplateOrExact(pattern, openAPIPath), nil
}

func matchGlob(pattern, s string) bool {
	// path.Match does not treat * as crossing '/', so convert * → ** style via regex.
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		case '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		// Fallback to path.Match for pathological patterns.
		ok, _ := path.Match(pattern, s)
		return ok
	}
	return re.MatchString(s)
}

func matchTemplateOrExact(pattern, openAPIPath string) bool {
	pSegs := splitPath(pattern)
	oSegs := splitPath(openAPIPath)
	if len(pSegs) != len(oSegs) {
		return false
	}
	for i := range pSegs {
		ps, os := pSegs[i], oSegs[i]
		if isTemplateSeg(ps) || isTemplateSeg(os) {
			continue
		}
		if ps != os {
			return false
		}
	}
	return true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func isTemplateSeg(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && len(s) > 2
}
