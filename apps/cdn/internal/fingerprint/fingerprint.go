package fingerprint

import (
	"fmt"
	"io"
	"os"
)

const (
	fnvOffsetBasis uint64 = 14695981039346656037
	fnvPrime       uint64 = 1099511628211
)

// Compute returns a stable FNV-1a fingerprint directly in Go.
func Compute(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("file fingerprint failed: %w", err)
	}
	defer file.Close()

	hash := fnvOffsetBasis
	buffer := make([]byte, 64*1024)
	for {
		count, readErr := file.Read(buffer)
		for _, value := range buffer[:count] {
			hash ^= uint64(value)
			hash *= fnvPrime
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", fmt.Errorf("file fingerprint failed: %w", readErr)
		}
	}
	return fmt.Sprintf("%016x", hash), nil
}
