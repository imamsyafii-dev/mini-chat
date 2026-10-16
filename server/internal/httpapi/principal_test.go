package httpapi

import (
	"context"
	"testing"
)

func TestPrincipalFromContext(
	t *testing.T,
) {
	tests := []struct {
		name          string
		context       context.Context
		wantPrincipal Principal
		wantOK        bool
	}{
		{
			name:    "missing principal",
			context: context.Background(),
		},
		{
			name: "valid principal",
			context: context.WithValue(
				context.Background(),
				authenticatedPrincipalContextKey,
				Principal{IdentityID: 42},
			),
			wantPrincipal: Principal{
				IdentityID: 42,
			},
			wantOK: true,
		},
		{
			name: "wrong value type",
			context: context.WithValue(
				context.Background(),
				authenticatedPrincipalContextKey,
				"not a principal",
			),
		},
		{
			name: "zero identity ID",
			context: context.WithValue(
				context.Background(),
				authenticatedPrincipalContextKey,
				Principal{IdentityID: 0},
			),
		},
		{
			name: "negative identity ID",
			context: context.WithValue(
				context.Background(),
				authenticatedPrincipalContextKey,
				Principal{IdentityID: -1},
			),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				principal, ok := PrincipalFromContext(
					test.context,
				)

				if ok != test.wantOK {
					t.Errorf(
						"ok = %t, want %t",
						ok,
						test.wantOK,
					)
				}

				if principal != test.wantPrincipal {
					t.Errorf(
						"principal = %#v, want %#v",
						principal,
						test.wantPrincipal,
					)
				}
			},
		)
	}
}
