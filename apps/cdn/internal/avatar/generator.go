package avatar

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultGeneratorScript  = "scripts/avatar_generator.py"
	defaultPythonExecutable = "python3"
)

// Generate delegates avatar rendering to the Python script. Keeping this Go
// layer small makes the CDN responsible for storage and HTTP only while the
// visual generation stays isolated in a dependency-free script.
func Generate(ownerID, script, pythonExecutable string) (string, error) {
	path := strings.TrimSpace(script)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("AVATAR_GENERATOR_SCRIPT"))
	}
	if path == "" {
		path = defaultGeneratorScript
	}
	path = resolveDefaultScript(path)
	python := strings.TrimSpace(pythonExecutable)
	if python == "" {
		python = strings.TrimSpace(os.Getenv("PYTHON_EXECUTABLE"))
	}
	if python == "" {
		python = defaultPythonExecutable
	}

	command := exec.Command(python, path, ownerID)
	var output bytes.Buffer
	var errors bytes.Buffer
	command.Stdout = &output
	command.Stderr = &errors
	if err := command.Run(); err != nil {
		details := strings.TrimSpace(errors.String())
		if details != "" {
			return "", fmt.Errorf("avatar script failed: %w: %s", err, details)
		}
		return "", fmt.Errorf("avatar script failed: %w", err)
	}

	svg := strings.TrimSpace(output.String())
	if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
		return "", fmt.Errorf("avatar script returned invalid SVG")
	}
	return svg, nil
}

func resolveDefaultScript(path string) string {
	if path != defaultGeneratorScript {
		return path
	}
	for _, candidate := range []string{
		path,
		filepath.Join("..", "..", path),
		filepath.Join("..", "..", "..", path),
		filepath.Join("..", "..", "..", "..", path),
		filepath.Join("apps", "cdn", path),
		"/usr/local/bin/avatar_generator.py",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}
