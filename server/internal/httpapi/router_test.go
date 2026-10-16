package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(
	t *testing.T,
) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	contentType := recorder.Header().Get(
		"Content-Type",
	)
	if !strings.HasPrefix(
		contentType,
		"application/json",
	) {
		t.Errorf(
			"Content-Type = %q, want application/json",
			contentType,
		)
	}

	var response healthResponse
	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if response.Status != "ok" {
		t.Errorf(
			"response status = %q, want %q",
			response.Status,
			"ok",
		)
	}
}

func TestHandlerRejectsUnsupportedHealthMethod(
	t *testing.T,
) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/health",
		nil,
	)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusMethodNotAllowed,
		)
	}
}

func TestHandlerReturnsNotFound(
	t *testing.T,
) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/unknown",
		nil,
	)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}
