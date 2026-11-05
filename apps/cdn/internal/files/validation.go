package files

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"card-arena/cdn/internal/imageinfo"
	"card-arena/cdn/internal/svgsafety"
)

const maxImageDimension = 8192

func firstFormFile(r *http.Request, names ...string) (multipart.File, *multipart.FileHeader, error) {
	for _, name := range names {
		file, header, err := r.FormFile(name)
		if err == nil {
			return file, header, nil
		}
	}
	return nil, nil, errors.New("file not found")
}

func detectFormat(file multipart.File) (string, string, error) {
	buffer := make([]byte, 512)
	count, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	detected := strings.ToLower(strings.Split(http.DetectContentType(buffer[:count]), ";")[0])
	if strings.HasPrefix(detected, "text/plain") {
		detected = "text/plain"
	}
	for _, format := range knownFormats {
		if format.contentType == detected {
			return format.contentType, format.extension, nil
		}
	}
	return "application/octet-stream", ".bin", nil
}

func contentTypeForExtension(extension string) string {
	for _, format := range knownFormats {
		if format.extension == strings.ToLower(extension) {
			return format.contentType
		}
	}
	return "application/octet-stream"
}

func isInlineContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "audio/") ||
		strings.HasPrefix(contentType, "video/") ||
		contentType == "application/pdf" ||
		contentType == "text/plain" ||
		contentType == "text/csv"
}

func validateImageAsset(filePath, contentType string) error {
	if contentType == "image/svg+xml" {
		return svgsafety.Validate(filePath)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil
	}

	info, err := imageinfo.Inspect(filePath)
	if err != nil {
		return err
	}
	if info.Format == "unknown" || info.Width <= 0 || info.Height <= 0 {
		return fmt.Errorf("image dimensions are unavailable")
	}
	if info.Width > maxImageDimension || info.Height > maxImageDimension {
		return fmt.Errorf(
			"image dimensions exceed %d pixels",
			maxImageDimension,
		)
	}
	return nil
}

func validPath(value string) bool {
	if value == "" || len(value) > 240 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." || !validSegment(segment) {
			return false
		}
	}
	return true
}

func validSegment(value string) bool {
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}
