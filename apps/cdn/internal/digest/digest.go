package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Compute returns a SHA-256 digest directly in Go.
func Compute(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("asset digest failed: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, 64*1024)
	for {
		count, readErr := file.Read(buffer)
		if count > 0 {
			_, _ = hash.Write(buffer[:count])
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", fmt.Errorf("asset digest failed: %w", readErr)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
