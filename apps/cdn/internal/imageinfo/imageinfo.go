package imageinfo

import (
	"encoding/binary"
	"fmt"
	"os"
)

type Info struct {
	Format string
	Width  int
	Height int
}

// Inspect reads image dimensions from headers directly in Go.
func Inspect(filePath string) (Info, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Info{}, fmt.Errorf("image info failed: %w", err)
	}
	if width, height, ok := pngSize(data); ok {
		return Info{"png", width, height}, nil
	}
	if width, height, ok := gifSize(data); ok {
		return Info{"gif", width, height}, nil
	}
	if width, height, ok := jpegSize(data); ok {
		return Info{"jpeg", width, height}, nil
	}
	if width, height, ok := webpSize(data); ok {
		return Info{"webp", width, height}, nil
	}
	return Info{Format: "unknown"}, nil
}

func pngSize(data []byte) (int, int, bool) {
	if len(data) < 24 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		return 0, 0, false
	}
	return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24])), true
}

func gifSize(data []byte) (int, int, bool) {
	if len(data) < 10 || (string(data[:6]) != "GIF87a" && string(data[:6]) != "GIF89a") {
		return 0, 0, false
	}
	return int(binary.LittleEndian.Uint16(data[6:8])), int(binary.LittleEndian.Uint16(data[8:10])), true
}

func jpegSize(data []byte) (int, int, bool) {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 0, 0, false
	}
	for index := 2; index+1 < len(data); {
		if data[index] != 0xff {
			index++
			continue
		}
		for index < len(data) && data[index] == 0xff {
			index++
		}
		if index >= len(data) {
			return 0, 0, false
		}
		marker := data[index]
		index++
		if marker == 0xd9 || marker == 0xda {
			return 0, 0, false
		}
		if marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if index+2 > len(data) {
			return 0, 0, false
		}
		segmentSize := int(binary.BigEndian.Uint16(data[index : index+2]))
		if segmentSize < 2 || index+segmentSize > len(data) {
			return 0, 0, false
		}
		if isJpegStartOfFrame(marker) && segmentSize >= 7 {
			height := int(binary.BigEndian.Uint16(data[index+3 : index+5]))
			width := int(binary.BigEndian.Uint16(data[index+5 : index+7]))
			return width, height, width > 0 && height > 0
		}
		index += segmentSize
	}
	return 0, 0, false
}

func isJpegStartOfFrame(marker byte) bool {
	return marker >= 0xc0 && marker <= 0xc3 || marker >= 0xc5 && marker <= 0xc7 ||
		marker >= 0xc9 && marker <= 0xcb || marker >= 0xcd && marker <= 0xcf
}

func webpSize(data []byte) (int, int, bool) {
	if len(data) < 30 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(data[12:16]) {
	case "VP8X":
		width := int(data[24]) | int(data[25])<<8 | int(data[26])<<16
		height := int(data[27]) | int(data[28])<<8 | int(data[29])<<16
		return width + 1, height + 1, true
	case "VP8 ":
		if data[23] == 0x9d && data[24] == 0x01 && data[25] == 0x2a {
			width := int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff)
			height := int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff)
			return width, height, width > 0 && height > 0
		}
	case "VP8L":
		if len(data) >= 25 && data[20] == 0x2f {
			dimensions := binary.LittleEndian.Uint32(data[21:25])
			width := int(dimensions&0x3fff) + 1
			height := int((dimensions>>14)&0x3fff) + 1
			return width, height, true
		}
	}
	return 0, 0, false
}
