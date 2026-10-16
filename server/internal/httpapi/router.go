package httpapi

import (
	"encoding/json"
	"net/http"
)

type Dependencies struct {
	Register Registerer
}

type healthResponse struct {
	Status string `json:"status"`
}

func NewHandler() http.Handler {
	return NewHandlerWithDependencies(
		Dependencies{},
	)
}

func NewHandlerWithDependencies(
	dependencies Dependencies,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler,
	)

	if dependencies.Register != nil {
		mux.Handle(
			"POST /auth/register",
			newRegisterHandler(
				dependencies.Register,
			),
		)
	}

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
