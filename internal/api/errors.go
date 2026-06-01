package api

import "net/http"

func writeError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}
