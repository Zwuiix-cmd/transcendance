package svgsafety

import (
	"fmt"
	"os"
	"strings"
)

var blockedTokens = []string{
	"<script",
	"<foreignobject",
	"<!doctype",
	"<!entity",
	"javascript:",
	"data:text/html",
	"url(http",
	"url(https",
	"href=\"http",
	"href='http",
	"xlink:href=\"http",
	"xlink:href='http",
	"onload=",
	"onerror=",
	"onclick=",
	"onmouseover=",
	"onfocus=",
	"onbegin=",
}

// Validate rejects executable or externally loaded SVG content in Go.
func Validate(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("SVG safety check failed: %w", err)
	}
	normalized := strings.ToLower(string(data))
	trimmed := strings.TrimLeft(normalized, " \t\r\n")
	if !strings.HasPrefix(trimmed, "<svg") {
		return fmt.Errorf("SVG document must start with an <svg> element")
	}
	for _, token := range blockedTokens {
		if strings.Contains(normalized, token) {
			return fmt.Errorf("blocked SVG token: %s", token)
		}
	}
	return nil
}
