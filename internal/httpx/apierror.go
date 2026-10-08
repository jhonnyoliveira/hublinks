package httpx

import (
	"encoding/json"
	"net/http"
)

// APIError escreve o envelope de erro de /api/v1:
// {"error":{"code","message","fields"?}}. fields é omitido quando vazio.
func APIError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	body := map[string]any{"code": code, "message": message}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": body})
}
