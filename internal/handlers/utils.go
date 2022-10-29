package handlers

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type Middleware func(http.Handler) http.HandlerFunc

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// checkForJSON checks if recieved data has type json as expected by the endpoint
func checkForJSON(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "400 - This endpoint accepts only jsons", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// checkForText checks if recieved data has type text as expected by the endpoint
func checkForText(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "text/plain" {
			http.Error(w, "400 - This endpoint accepts only plain text", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// unpackGZIP unzips response from the service
func unpackGZIP(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" && r.Header.Get("Content-Encoding") != "" {
			http.Error(w, "400 - Only gzip encoding is allowed", http.StatusBadRequest)
			return
		} else if r.Header.Get("Content-Encoding") != "gzip" {
			next.ServeHTTP(w, r)
			return
		}

		rw, err := gzip.NewReader(r.Body)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		r.Body = rw
		next.ServeHTTP(w, r)
	})
}

// packGZIP zips response from the service
func packGZIP(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

// Conveyor allows to pack middlware one after another
func Conveyor(h http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	for _, middleware := range middlewares {
		h = middleware(h)
	}
	return h
}
