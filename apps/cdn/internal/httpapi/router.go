package httpapi

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"card-arena/cdn/internal/files"
)

// NewRouter wires public reads and authenticated internal mutations.
func NewRouter(store *files.Store, internalKey, staticAssetsDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler(store))
	metrics := newMetrics()
	mux.Handle("GET /metrics", metrics)
	mux.Handle("GET /files/{filePath...}", publicFilesHandler(store, staticAssetsDir))
	mux.Handle("POST /v1/files/{namespace}/{ownerID}/avatar/random",
		requireKey(internalKey, http.HandlerFunc(store.RandomAvatar)))
	mux.Handle("POST /v1/files/{namespace}/{ownerID}/{fileKey...}",
		requireKey(internalKey, http.HandlerFunc(store.Upload)))
	mux.Handle("DELETE /v1/files/{namespace}/{ownerID}/{fileKey...}",
		requireKey(internalKey, http.HandlerFunc(store.Remove)))
	return logging(mux, metrics)
}

func publicFilesHandler(store *files.Store, staticAssetsDir string) http.Handler {
	unoAssets := unoAssetsHandler(staticAssetsDir)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/files/uno/") {
			unoAssets.ServeHTTP(writer, request)
			return
		}
		parts := strings.SplitN(request.PathValue("filePath"), "/", 3)
		if len(parts) != 3 {
			writeError(writer, http.StatusBadRequest, "invalid file path")
			return
		}
		request.SetPathValue("namespace", parts[0])
		request.SetPathValue("ownerID", parts[1])
		request.SetPathValue("fileKey", parts[2])
		store.Serve(writer, request)
	})
}

func unoAssetsHandler(root string) http.Handler {
	root, _ = filepath.Abs(root)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "public, max-age=86400")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		relative := strings.TrimPrefix(request.URL.Path, "/files/uno/")
		if relative == "" {
			relative = "index.html"
		}
		filePath := filepath.Join(root, filepath.FromSlash(relative))
		relativeToRoot, err := filepath.Rel(root, filePath)
		if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeToRoot) {
			writeError(writer, http.StatusBadRequest, "invalid asset path")
			return
		}
		file, err := os.Open(filePath)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(writer, request)
			return
		}
		extension := strings.ToLower(filepath.Ext(filePath))
		if contentType := unoAssetContentType(extension); contentType != "" {
			writer.Header().Set("Content-Type", contentType)
		}
		http.ServeContent(writer, request, filepath.Base(filePath), info.ModTime(), file)
	})
}

func unoAssetContentType(extension string) string {
	switch extension {
	case ".glb":
		return "model/gltf-binary"
	case ".hdr":
		return "image/vnd.radiance"
	case ".hlsl":
		return "text/plain; charset=utf-8"
	default:
		return mime.TypeByExtension(extension)
	}
}
