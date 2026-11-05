package result

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
	"card-arena/api/internal/settlement"
)

type Route struct {
	route.Route
}

type resultServer interface {
	Database() *sql.DB
	ClientKey(*http.Request) string
	RecordAudit(string, string, string, string, string, string, string, string, string)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(resultServer)
	if !ok {
		route.NotImplemented(context)
		return
	}
	var input gameResultRequest
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	tx, err := server.Database().Begin()
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	outcome, err := settlement.Apply(tx, settlement.Result{GameID: context.Param("id"), Participants: input.Participants}, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, settlement.ErrGameNotFound):
			writeError(context.Writer, http.StatusNotFound, "game_not_found", "Game was not found")
		case errors.Is(err, settlement.ErrGameNotActive):
			writeError(context.Writer, http.StatusConflict, "game_not_active", "The game is not ready to be settled")
		default:
			var validationErr *settlement.ValidationError
			if errors.As(err, &validationErr) {
				writeError(context.Writer, http.StatusBadRequest, "invalid_game_result", validationErr.Message)
			} else {
				writeDatabaseError(context.Writer, err)
			}
		}
		return
	}
	if err = tx.Commit(); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	committed = true
	participants := make([]model.GameResultParticipant, 0, len(outcome.Participants))
	for _, participant := range outcome.Participants {
		participants = append(participants, model.GameResultParticipant{UserID: participant.UserID, Position: participant.Position, Score: participant.Score, EloDelta: participant.EloDelta})
	}
	server.RecordAudit("", "game-service", "game.result.settled", "game", context.Param("id"), "mode="+outcome.Mode, "info", server.ClientKey(context.Request), context.Request.UserAgent())
	writeJSON(context.Writer, http.StatusOK, model.GameResultResponse{GameID: outcome.GameID, Mode: outcome.Mode, Status: outcome.Status, EndedAt: outcome.EndedAt, AlreadySettled: outcome.AlreadySettled, Participants: participants})
}

type gameResultRequest struct {
	Participants []settlement.Participant `json:"participants"`
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return false
	}
	return true
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
	writeError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/result", route.AccessGameService),
}
