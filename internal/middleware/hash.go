package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

type responseRecorder struct {
	http.ResponseWriter
	body        *bytes.Buffer
	wroteHeader bool
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(statusCode)
}

func HashMiddleware(next http.Handler, key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		bodyBytes, err := io.ReadAll(r.Body)

		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		h := sha256.New()
		h.Write(bodyBytes)
		h.Write([]byte(key))
		expectedHash := hex.EncodeToString(h.Sum(nil))

		if r.Header.Get("HashSHA256") != expectedHash {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "hash mismatch"}`))
			return
		}

		recorder := &responseRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(recorder, r)

		hResp := sha256.New()
		hResp.Write(recorder.body.Bytes())
		hResp.Write([]byte(key))
		respHash := hex.EncodeToString(hResp.Sum(nil))

		w.Header().Set("HashSHA256", respHash)

		if !recorder.wroteHeader {
			w.WriteHeader(http.StatusOK)
		}
		w.Write(recorder.body.Bytes())
	})
}
