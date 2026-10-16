package httpapi

import "context"

type Principal struct {
	IdentityID int64
}

type principalContextKey struct{}

var authenticatedPrincipalContextKey principalContextKey

func contextWithPrincipal(
	ctx context.Context,
	principal Principal,
) context.Context {
	return context.WithValue(
		ctx,
		authenticatedPrincipalContextKey,
		principal,
	)
}

func PrincipalFromContext(
	ctx context.Context,
) (Principal, bool) {
	principal, ok := ctx.Value(
		authenticatedPrincipalContextKey,
	).(Principal)
	if !ok || principal.IdentityID <= 0 {
		return Principal{}, false
	}

	return principal, true
}
