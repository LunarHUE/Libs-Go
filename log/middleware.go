package log

import (
	"net/http"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LogRequest() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			loggingResponseWriter := NewLoggingResponseWriter(w)
			next.ServeHTTP(loggingResponseWriter, r)

			method := r.Method
			path := r.URL.Path
			status := loggingResponseWriter.statusCode

			Requestf("%v %s %s", status, method, path)
		})
	}
}
