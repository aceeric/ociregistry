package impl

import (
	"encoding/json"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// These helpers write responses exactly the way the Echo context methods they replace did
// (ctx.JSON, ctx.String, ctx.Blob, ctx.Stream), so clients see the same status codes,
// Content-Type values and bodies as before.

const (
	mimeJSON      = "application/json"
	mimeTextPlain = "text/plain; charset=UTF-8"
)

// setContentType sets the Content-Type header unless the handler already set one.
func setContentType(w http.ResponseWriter, contentType string) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", contentType)
	}
}

// writeJSON writes v as JSON followed by a newline.
func writeJSON(w http.ResponseWriter, code int, v any) {
	setContentType(w, mimeJSON)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Debugf("error writing JSON response: %s", err)
	}
}

// writeString writes s as plain text.
func writeString(w http.ResponseWriter, code int, s string) {
	writeBlob(w, code, mimeTextPlain, []byte(s))
}

// writeBlob writes b with the passed content type.
func writeBlob(w http.ResponseWriter, code int, contentType string, b []byte) {
	setContentType(w, contentType)
	w.WriteHeader(code)
	if _, err := w.Write(b); err != nil {
		log.Debugf("error writing response: %s", err)
	}
}

// writeStream copies rdr to the response with the passed content type.
func writeStream(w http.ResponseWriter, code int, contentType string, rdr io.Reader) {
	setContentType(w, contentType)
	w.WriteHeader(code)
	if _, err := io.Copy(w, rdr); err != nil {
		log.Debugf("error streaming response: %s", err)
	}
}
