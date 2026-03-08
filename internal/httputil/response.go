package httputil

import (
	"encoding/json"
	"net/http"
)

// ErrorEnvelope is the standard error response format.
type ErrorEnvelope struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, msg, code string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error:   msg,
		Code:    code,
		Details: details,
	})
}
