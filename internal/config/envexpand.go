package config

import (
	"fmt"
	"os"
	"regexp"
)

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvStrict replaces ${ENV_NAME} using os.LookupEnv.
// Missing variables return an error that includes fieldPath.
func expandEnvStrict(s, fieldPath string) (string, error) {
	var firstErr error
	out := envVarPattern.ReplaceAllStringFunc(s, func(m string) string {
		if firstErr != nil {
			return m
		}
		sub := envVarPattern.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		name := sub[1]
		val, ok := os.LookupEnv(name)
		if !ok {
			firstErr = fmt.Errorf("%s: environment variable %q is not set", fieldPath, name)
			return m
		}
		return val
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}
