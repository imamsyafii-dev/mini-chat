package httpapi

import (
	"context"
	"net/http"
	"strings"
)

type AccessTokenVerifier interface {
	Verify(
		ctx context.Context,
		raw string,
	) (int64, error)
}

func requireAuthentication(
	verifier AccessTokenVerifier,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			authorizationHeaders := request.Header.Values(
				"Authorization",
			)
			if len(authorizationHeaders) != 1 {
				writeUnauthorized(writer)

				return
			}

			parts := strings.Fields(
				authorizationHeaders[0],
			)
			if len(parts) != 2 ||
				!strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(writer)

				return
			}

			identityID, err := verifier.Verify(
				request.Context(),
				parts[1],
			)
			if err != nil || identityID <= 0 {
				writeUnauthorized(writer)

				return
			}

			request = request.WithContext(
				contextWithPrincipal(
					request.Context(),
					Principal{
						IdentityID: identityID,
					},
				),
			)

			next.ServeHTTP(writer, request)
		},
	)
}

func writeUnauthorized(
	writer http.ResponseWriter,
) {
	writer.Header().Set(
		"WWW-Authenticate",
		"Bearer",
	)
	writeJSON(
		writer,
		http.StatusUnauthorized,
		errorResponse{
			Error: "unauthorized",
		},
	)
}
