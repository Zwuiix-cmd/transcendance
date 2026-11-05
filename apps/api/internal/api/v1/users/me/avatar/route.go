package avatar

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type avatarDependencies interface {
	Database() *sql.DB
	Profile(string, bool) (model.User, error)
	UserID(any) (string, bool)
	CDNURL() string
	InternalCDNAPIKey() string
	HTTPClient() *http.Client
}

func Handle(context *route.Context) {
	server, ok := context.Server.(avatarDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	userID, ok := server.UserID(context.User)
	if !ok {
		writeError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return
	}
	if context.Request.ContentLength > 6<<20 {
		writeError(context.Writer, http.StatusRequestEntityTooLarge, "avatar_too_large", "Avatar is too large")
		return
	}
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, 6<<20)
	if err := context.Request.ParseMultipartForm(6 << 20); err != nil {
		writeError(context.Writer, http.StatusBadRequest, "invalid_upload", "Invalid avatar upload")
		return
	}
	file, header, err := context.Request.FormFile("avatar")
	if err != nil {
		writeError(context.Writer, http.StatusBadRequest, "avatar_required", "An avatar file is required")
		return
	}
	defer file.Close()
	if header.Size > 5<<20 {
		writeError(context.Writer, http.StatusRequestEntityTooLarge, "avatar_too_large", "Avatar is too large")
		return
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("avatar", header.Filename)
	if err == nil {
		_, err = io.Copy(part, file)
	}
	if closeErr := form.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		writeError(context.Writer, http.StatusBadRequest, "invalid_upload", "Could not read avatar")
		return
	}
	endpoint := server.CDNURL() + "/v1/files/avatars/" + url.PathEscape(userID) + "/avatar"
	proxy, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		writeError(context.Writer, http.StatusBadGateway, "cdn_unavailable", "Avatar storage is unavailable")
		return
	}
	proxy.Header.Set("Content-Type", form.FormDataContentType())
	proxy.Header.Set("X-Internal-API-Key", server.InternalCDNAPIKey())
	response, err := server.HTTPClient().Do(proxy)
	if err != nil {
		writeError(context.Writer, http.StatusBadGateway, "cdn_unavailable", "Avatar storage is unavailable")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeError(context.Writer, http.StatusBadGateway, "cdn_rejected", "Avatar storage rejected the file")
		return
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil || payload.URL == "" {
		writeError(context.Writer, http.StatusBadGateway, "cdn_invalid_response", "Avatar storage returned an invalid response")
		return
	}
	if _, err = server.Database().Exec(`UPDATE users SET avatar_url=? WHERE id=? AND deleted_at IS NULL`, payload.URL, userID); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	profile, err := server.Profile(userID, true)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	writeJSON(context.Writer, http.StatusOK, profile)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPut, "/api/v1/users/me/avatar", route.AccessUser),
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"code": code, "message": message, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

func writeDatabaseError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "not_found", "Resource was not found")
		return
	}
	slog.Error("database request failed", "error", err)
	writeError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}
