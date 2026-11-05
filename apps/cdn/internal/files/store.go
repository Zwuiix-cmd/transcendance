package files

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"card-arena/cdn/internal/avatar"
	"card-arena/cdn/internal/digest"
	"card-arena/cdn/internal/fingerprint"
	"card-arena/cdn/internal/magic"
)

// Store owns the persistent file namespace used by the CDN.
type Store struct {
	root            string
	publicURL       string
	generatorScript string
	locksMu         sync.Mutex
	locks           map[string]*sync.Mutex
	metadataMu      sync.RWMutex
	metadata        map[string]fileMetadata
}

type fileMetadata struct {
	digest      string
	contentType string
	size        int64
}

// NewStore creates the storage root if it does not exist yet.
func NewStore(root, publicURL string) (*Store, error) {
	return NewStoreWithGenerator(root, publicURL, "")
}

// NewStoreWithGenerator creates a store with an explicit avatar script.
func NewStoreWithGenerator(root, publicURL, generatorScript string) (*Store, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &Store{
		root:            root,
		publicURL:       strings.TrimRight(publicURL, "/"),
		generatorScript: generatorScript,
		locks:           make(map[string]*sync.Mutex),
		metadata:        make(map[string]fileMetadata),
	}, nil
}

// Ready verifies that the storage root is available and writable.
func (store *Store) Ready() error {
	info, err := os.Stat(store.root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("cdn storage root is not a directory")
	}
	temporary, err := os.CreateTemp(store.root, ".ready-*")
	if err != nil {
		return err
	}
	path := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return os.Remove(path)
}

func (store *Store) lockFor(namespace, ownerID, fileKey string) func() {
	key := namespace + "\x00" + ownerID + "\x00" + fileKey
	store.locksMu.Lock()
	lock := store.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		store.locks[key] = lock
	}
	store.locksMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

// Serve returns a public file and lazily creates a default avatar when needed.
func (store *Store) Serve(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	ownerID := r.PathValue("ownerID")
	fileKey := r.PathValue("fileKey")
	if !validPath(namespace) || !validPath(ownerID) || !validPath(fileKey) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	unlock := store.lockFor(namespace, ownerID, fileKey)
	defer unlock()
	path, contentType, found := store.existingForAccept(namespace, ownerID, fileKey, r.Header.Get("Accept"))
	if !found && namespace == "avatars" && fileKey == "avatar" {
		var err error
		path, err = store.writeGeneratedAvatar(ownerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create default avatar")
			return
		}
		contentType = "image/svg+xml"
		found = true
	}
	if !found {
		writeError(w, http.StatusNotFound, "file was not found")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(contentType, "image/") {
		w.Header().Set("Vary", "Accept")
	}
	if contentDigest := store.contentDigest(path); contentDigest != "" {
		etag := "\"" + contentDigest + "\""
		w.Header().Set("ETag", etag)
		w.Header().Set("X-Content-Digest", "sha-256="+contentDigest)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	generatedAvatar := namespace == "avatars" && fileKey == "avatar" && contentType == "image/svg+xml"
	if !isInlineContentType(contentType) && !generatedAvatar {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeFile(w, r, path)
}

// Upload stores a multipart file. Rename publishes the new version before old
// versions are cleaned up, so a crash cannot remove the only valid version.
func (store *Store) Upload(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	ownerID := r.PathValue("ownerID")
	fileKey := r.PathValue("fileKey")
	if !validPath(namespace) || !validPath(ownerID) || !validPath(fileKey) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	limit := maxFileBytes
	if namespace == "avatars" && fileKey == "avatar" {
		limit = maxAvatarBytes
	}
	requestLimit := limit + (1 << 20)
	if r.ContentLength > requestLimit {
		writeError(w, http.StatusRequestEntityTooLarge, "file is too large")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, requestLimit)
	if err := r.ParseMultipartForm(requestLimit); err != nil {
		writeError(w, http.StatusBadRequest, "invalid file upload")
		return
	}
	file, header, err := firstFormFile(r, "file", "avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "a file field named file is required")
		return
	}
	defer file.Close()
	contentType, extension, err := detectFormat(file)
	if err != nil {
		writeError(w, http.StatusUnsupportedMediaType, "the file type is not supported")
		return
	}
	if header.Size > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "file is too large")
		return
	}

	unlock := store.lockFor(namespace, ownerID, fileKey)
	defer unlock()
	basePath := store.filePath(namespace, ownerID, fileKey)
	storageDirectory := filepath.Dir(basePath)
	if err := os.MkdirAll(storageDirectory, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "could not prepare file storage")
		return
	}
	temporary, err := os.CreateTemp(storageDirectory, ".upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store file")
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		_ = temporary.Close()
		writeError(w, http.StatusBadRequest, "could not read file")
		return
	}
	written, copyErr := io.CopyN(temporary, file, limit+1)
	closeErr := temporary.Close()
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		writeError(w, http.StatusBadRequest, "could not read file")
		return
	}
	if closeErr != nil || written > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "file is too large")
		return
	}
	if detectedFormat, detectedErr := magic.Detect(temporaryPath); detectedErr == nil {
		contentType = detectedFormat.ContentType
		extension = detectedFormat.Extension
	}
	if err := validateImageAsset(temporaryPath, contentType); err != nil {
		writeError(w, http.StatusUnsupportedMediaType, "file failed safety validation")
		return
	}
	destination := basePath + extension
	if err := os.Rename(temporaryPath, destination); err != nil {
		writeError(w, http.StatusInternalServerError, "could not publish file")
		return
	}
	contentDigest := store.contentDigest(destination)
	store.remember(destination, contentType, contentDigest, written)
	store.removeExistingExcept(namespace, ownerID, fileKey, destination)
	store.generateVariants(destination, basePath, contentType)
	writeJSON(w, http.StatusCreated, Response{Namespace: namespace, OwnerID: ownerID, Key: fileKey, URL: store.versionedURL(namespace, ownerID, fileKey), ContentType: contentType, Size: written, Digest: contentDigest})
	slog.Info("file uploaded", "namespace", namespace, "ownerId", ownerID, "key", fileKey, "contentType", contentType, "bytes", written)
}

// RandomAvatar ensures that an account has a generated avatar.
func (store *Store) RandomAvatar(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	ownerID := r.PathValue("ownerID")
	if namespace != "avatars" || !validPath(ownerID) {
		writeError(w, http.StatusBadRequest, "random files are only available for avatars")
		return
	}
	unlock := store.lockFor(namespace, ownerID, "avatar")
	defer unlock()
	path, _, found := store.existing(namespace, ownerID, "avatar")
	if !found {
		var err error
		path, err = store.writeGeneratedAvatar(ownerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create default avatar")
			return
		}
	}
	contentDigest := store.contentDigest(path)
	writeJSON(w, http.StatusCreated, Response{Namespace: "avatars", OwnerID: ownerID, Key: "avatar", URL: store.versionedURL("avatars", ownerID, "avatar"), ContentType: "image/svg+xml", Size: 0, Digest: contentDigest})
}

// Remove deletes all versions of a file key.
func (store *Store) Remove(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	ownerID := r.PathValue("ownerID")
	fileKey := r.PathValue("fileKey")
	if !validPath(namespace) || !validPath(ownerID) || !validPath(fileKey) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}
	unlock := store.lockFor(namespace, ownerID, fileKey)
	defer unlock()
	store.removeExisting(namespace, ownerID, fileKey)
	w.WriteHeader(http.StatusNoContent)
}

func (store *Store) existing(namespace, ownerID, fileKey string) (string, string, bool) {
	return store.existingForAccept(namespace, ownerID, fileKey, "")
}

func (store *Store) existingForAccept(namespace, ownerID, fileKey, accept string) (string, string, bool) {
	basePath := store.filePath(namespace, ownerID, fileKey)
	directory := filepath.Dir(basePath)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", "", false
	}
	prefix := filepath.Base(basePath) + "."
	preferred := ""
	if acceptsMediaType(accept, "image/avif") {
		preferred = ".avif"
	} else if acceptsMediaType(accept, "image/webp") {
		preferred = ".webp"
	}
	if preferred != "" {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.Contains(entry.Name(), ".variant.") || filepath.Ext(entry.Name()) != preferred {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			return path, contentTypeForExtension(preferred), true
		}
	}
	var newest os.FileInfo
	var newestPath string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if strings.Contains(entry.Name(), ".variant.") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, err := entry.Info()
		if err != nil || (newest != nil && !info.ModTime().After(newest.ModTime())) {
			continue
		}
		newest, newestPath = info, path
	}
	if newestPath != "" {
		return newestPath, contentTypeForExtension(filepath.Ext(newestPath)), true
	}
	return "", "", false
}

func acceptsMediaType(header, mediaType string) bool {
	for _, part := range strings.Split(strings.ToLower(header), ",") {
		pieces := strings.Split(part, ";")
		media := strings.TrimSpace(pieces[0])
		if media != mediaType && media != "*/*" {
			continue
		}
		accepted := true
		for _, parameter := range pieces[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) == 2 && strings.TrimSpace(keyValue[0]) == "q" && strings.TrimSpace(keyValue[1]) == "0" {
				accepted = false
				break
			}
		}
		if accepted {
			return true
		}
	}
	return false
}

// generateVariants uses optional system encoders when present. Uploads never
// depend on them: the original remains the canonical representation if an
// encoder is unavailable in a minimal container.
func (store *Store) generateVariants(source, basePath, contentType string) {
	if !strings.HasPrefix(contentType, "image/") || contentType == "image/svg+xml" {
		return
	}
	directory := filepath.Dir(source)
	for _, variant := range []struct {
		command string
		args    func(string, string) []string
		ext     string
	}{
		{command: "cwebp", args: func(input, output string) []string { return []string{"-quiet", "-q", "82", input, "-o", output} }, ext: ".webp"},
		{command: "avifenc", args: func(input, output string) []string { return []string{"--min", "20", "--max", "35", input, output} }, ext: ".avif"},
	} {
		if filepath.Ext(source) == variant.ext {
			continue
		}
		if _, err := exec.LookPath(variant.command); err != nil {
			continue
		}
		temporary, err := os.CreateTemp(directory, ".variant-*")
		if err != nil {
			continue
		}
		temporaryPath := temporary.Name()
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		destination := basePath + ".variant" + variant.ext
		command := exec.Command(variant.command, variant.args(source, temporaryPath)...)
		if err := command.Run(); err != nil {
			_ = os.Remove(temporaryPath)
			continue
		}
		if info, err := os.Stat(temporaryPath); err != nil || info.Size() == 0 {
			_ = os.Remove(temporaryPath)
			continue
		}
		if err := os.Rename(temporaryPath, destination); err != nil {
			_ = os.Remove(temporaryPath)
			continue
		}
		store.remember(destination, contentTypeForExtension(variant.ext), store.contentDigest(destination), 0)
	}
}

func (store *Store) writeGeneratedAvatar(ownerID string) (string, error) {
	namespace, fileKey := "avatars", "avatar"
	destination := store.filePath(namespace, ownerID, fileKey) + ".svg"
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".generated-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	generated, err := avatar.Generate(ownerID, store.generatorScript, "")
	if err != nil {
		_ = temporary.Close()
		return "", err
	}
	if _, err = temporary.WriteString(generated); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err = temporary.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(temporaryPath, destination); err != nil {
		return "", err
	}
	digestValue := store.contentDigest(destination)
	store.remember(destination, "image/svg+xml", digestValue, int64(len(generated)))
	store.removeExistingExcept(namespace, ownerID, fileKey, destination)
	return destination, nil
}

func (store *Store) removeExisting(namespace, ownerID, fileKey string) {
	store.removeExistingExcept(namespace, ownerID, fileKey, "")
}

func (store *Store) removeExistingExcept(namespace, ownerID, fileKey, keep string) {
	basePath := store.filePath(namespace, ownerID, fileKey)
	directory := filepath.Dir(basePath)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	prefix := filepath.Base(basePath) + "."
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if path == keep {
			continue
		}
		_ = os.Remove(path)
		store.metadataMu.Lock()
		delete(store.metadata, path)
		store.metadataMu.Unlock()
	}
}

func (store *Store) filePath(namespace, ownerID, fileKey string) string {
	return filepath.Join(store.root, filepath.FromSlash(namespace), filepath.FromSlash(ownerID), filepath.FromSlash(fileKey))
}

func (store *Store) versionedURL(namespace, ownerID, fileKey string) string {
	version := strconv.FormatInt(time.Now().UnixNano(), 10)
	if path, _, found := store.existing(namespace, ownerID, fileKey); found {
		if value := store.contentDigest(path); value != "" {
			version = value
		} else if value, err := fingerprint.Compute(path); err == nil {
			version = value
		}
	}
	parts := []string{store.publicURL, url.PathEscape(namespace), url.PathEscape(ownerID)}
	for _, part := range strings.Split(fileKey, "/") {
		parts = append(parts, url.PathEscape(part))
	}
	return strings.Join(parts, "/") + "?v=" + version
}

func (store *Store) contentDigest(path string) string {
	store.metadataMu.RLock()
	if metadata, ok := store.metadata[path]; ok && metadata.digest != "" {
		store.metadataMu.RUnlock()
		return metadata.digest
	}
	store.metadataMu.RUnlock()
	value, err := digest.Compute(path)
	if err != nil {
		slog.Warn("could not compute asset digest", "path", path, "error", err)
		return ""
	}
	store.metadataMu.Lock()
	metadata := store.metadata[path]
	metadata.digest = value
	if metadata.size == 0 {
		if info, statErr := os.Stat(path); statErr == nil {
			metadata.size = info.Size()
		}
	}
	store.metadata[path] = metadata
	store.metadataMu.Unlock()
	return value
}

func (store *Store) remember(path, contentType, digestValue string, size int64) {
	store.metadataMu.Lock()
	store.metadata[path] = fileMetadata{digest: digestValue, contentType: contentType, size: size}
	store.metadataMu.Unlock()
}
