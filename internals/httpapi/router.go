package httpapi

import "net/http"

type handler struct {
	readings *readingStore
}

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	h := &handler{
		readings: newReadingStore(),
	}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /devices/{id}", deviceHandler)
	mux.HandleFunc("GET /devices/{id}/readings/latest", h.latestHandler)
	mux.HandleFunc("POST /devices/{id}/readings", h.createReadingHandler)

	return mux
}
