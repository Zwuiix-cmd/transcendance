package magic

import (
	"bytes"
	"fmt"
	"os"
)

type Format struct {
	ContentType string
	Extension   string
}

var formats = map[string]Format{
	"png":    {ContentType: "image/png", Extension: ".png"},
	"jpeg":   {ContentType: "image/jpeg", Extension: ".jpg"},
	"webp":   {ContentType: "image/webp", Extension: ".webp"},
	"gif":    {ContentType: "image/gif", Extension: ".gif"},
	"svg":    {ContentType: "image/svg+xml", Extension: ".svg"},
	"pdf":    {ContentType: "application/pdf", Extension: ".pdf"},
	"text":   {ContentType: "text/plain", Extension: ".txt"},
	"json":   {ContentType: "application/json", Extension: ".json"},
	"mp3":    {ContentType: "audio/mpeg", Extension: ".mp3"},
	"ogg":    {ContentType: "audio/ogg", Extension: ".ogg"},
	"wav":    {ContentType: "audio/wav", Extension: ".wav"},
	"mp4":    {ContentType: "video/mp4", Extension: ".mp4"},
	"webm":   {ContentType: "video/webm", Extension: ".webm"},
	"zip":    {ContentType: "application/zip", Extension: ".zip"},
	"binary": {ContentType: "application/octet-stream", Extension: ".bin"},
}

// Detect inspects file signatures directly in Go.
func Detect(filePath string) (Format, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Format{}, fmt.Errorf("file magic failed: %w", err)
	}
	format := detect(data)
	value, ok := formats[format]
	if !ok {
		return Format{}, fmt.Errorf("file magic returned an unknown format %q", format)
	}
	return value, nil
}

func detect(data []byte) string {
	starts := func(signature []byte) bool { return bytes.HasPrefix(data, signature) }
	switch {
	case starts([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "jpeg"
	case starts([]byte("GIF87a")), starts([]byte("GIF89a")):
		return "gif"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "webp"
	case starts([]byte("%PDF-")): // why not?
		return "pdf"
	case starts([]byte("PK\x03\x04")), starts([]byte("PK\x05\x06")), starts([]byte("PK\x07\x08")):
		return "zip"
	case starts([]byte("ID3")), len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0:
		return "mp3"
	case starts([]byte("OggS")):
		return "ogg"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")):
		return "wav"
	case len(data) >= 8 && bytes.Equal(data[4:8], []byte("ftyp")):
		return "mp4"
	case starts([]byte{0x1a, 0x45, 0xdf, 0xa3}):
		return "webm"
	}

	if isText(data) {
		trimmed := bytes.TrimLeft(data, " \t\r\n")
		switch {
		case bytes.HasPrefix(trimmed, []byte("{")), bytes.HasPrefix(trimmed, []byte("[")):
			return "json"
		case bytes.HasPrefix(trimmed, []byte("<svg")):
			return "svg"
		default:
			return "text"
		}
	}
	return "binary"
}

func isText(data []byte) bool {
	for _, value := range data {
		if value == '\t' || value == '\n' || value == '\r' {
			continue
		}
		if value < 0x20 || value > 0x7e {
			return false
		}
	}
	return true
}
