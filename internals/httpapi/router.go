package httpapi

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /devices/{id}", deviceHandler)
	mux.HandleFunc("GET /devices/{id}/readings/latest", latestHandler)

	return mux
}
