package httpapi

import (
	"encoding/json"
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	return mux
}

func healthHandler(
	writer http.ResponseWriter,
	_ *http.Request,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json",
	)
	writer.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(writer).Encode(
		healthResponse{
			Status: "ok",
		},
	)
}
