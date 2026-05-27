package handler

import (
	"encoding/json"
	"net/http"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/config"
	"github.com/chasedputnam/go-music-gen/internal/schema"
	"github.com/chasedputnam/go-music-gen/internal/tmpdir"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	Cfg      *config.Settings
	Bridge   *bridge.Bridge
	Registry *tmpdir.Registry
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a {"detail": msg} JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, schema.ErrorResponse{Detail: msg})
}
