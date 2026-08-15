package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 8 * 1024

func decodeJSONBody(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)

	// Consume the complete bounded body before decoding so a valid JSON prefix
	// cannot hide oversized or malformed trailing content.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		// Preserve the API's existing malformed-body response for read failures.
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}

	if err := json.Unmarshal(body, destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}

	return true
}

func writeError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}
