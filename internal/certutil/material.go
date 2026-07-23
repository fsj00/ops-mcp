package certutil

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// ResolveMaterial returns certificate/key bytes from inline content or a file path.
//
// Rules:
//   - content and file are mutually exclusive
//   - both empty → (nil, nil) so callers may treat the material as optional
//   - file → os.ReadFile
//   - content containing "-----BEGIN" → used as PEM/key text
//   - otherwise content is standard base64 of the file bytes
func ResolveMaterial(content, file, label string) ([]byte, error) {
	content = strings.TrimSpace(content)
	file = strings.TrimSpace(file)
	switch {
	case content != "" && file != "":
		return nil, fmt.Errorf("%s: set either content or file, not both", label)
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("%s: read file %q: %w", label, file, err)
		}
		return b, nil
	case content != "":
		return decodeContent(content, label)
	default:
		return nil, nil
	}
}

func decodeContent(content, label string) ([]byte, error) {
	if strings.Contains(content, "-----BEGIN") {
		return []byte(content), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid base64 content: %w", label, err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("%s: base64 content decoded to empty", label)
	}
	return decoded, nil
}
