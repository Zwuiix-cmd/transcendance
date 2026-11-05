package files

const (
	maxFileBytes   int64 = 25 << 20
	maxAvatarBytes int64 = 5 << 20
)

type fileFormat struct {
	contentType string
	extension   string
}

var knownFormats = []fileFormat{
	{contentType: "image/png", extension: ".png"},
	{contentType: "image/jpeg", extension: ".jpg"},
	{contentType: "image/webp", extension: ".webp"},
	{contentType: "image/avif", extension: ".avif"},
	{contentType: "image/gif", extension: ".gif"},
	{contentType: "image/svg+xml", extension: ".svg"},
	{contentType: "application/pdf", extension: ".pdf"},
	{contentType: "text/plain", extension: ".txt"},
	{contentType: "text/csv", extension: ".csv"},
	{contentType: "application/json", extension: ".json"},
	{contentType: "audio/mpeg", extension: ".mp3"},
	{contentType: "audio/ogg", extension: ".ogg"},
	{contentType: "audio/wav", extension: ".wav"},
	{contentType: "video/mp4", extension: ".mp4"},
	{contentType: "video/webm", extension: ".webm"},
	{contentType: "application/zip", extension: ".zip"},
	{contentType: "application/octet-stream", extension: ".bin"},
}

// Response is returned by internal file mutations.
type Response struct {
	Namespace   string `json:"namespace"`
	OwnerID     string `json:"ownerId"`
	Key         string `json:"key"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Digest      string `json:"digest,omitempty"`
}
