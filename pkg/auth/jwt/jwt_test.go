//
// Copyright (C) 2026 IOTech Ltd
//

package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signTestToken(t *testing.T, claims jwt.Claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-key"))
	require.NoError(t, err)
	return token
}

func TestGetExpiresAtFromRequest(t *testing.T) {
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	tokenWithExp := signTestToken(t, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)})

	tests := []struct {
		name     string
		auth     string
		expected time.Time
		ok       bool
	}{
		{name: "Bearer JWT with exp", auth: "Bearer " + tokenWithExp, expected: exp, ok: true},
		{name: "Lower-case bearer prefix", auth: "bearer " + tokenWithExp, expected: exp, ok: true},
		{name: "Expired JWT still returns its exp", auth: "Bearer " + signTestToken(t, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp.Add(-2 * time.Hour))}), expected: exp.Add(-2 * time.Hour), ok: true},
		{name: "JWT without exp claim", auth: "Bearer " + signTestToken(t, jwt.RegisteredClaims{Issuer: "test"})},
		{name: "Missing header", auth: ""},
		{name: "Bearer prefix only", auth: "Bearer "},
		{name: "Non-Bearer scheme", auth: "Basic dXNlcjpwYXNz"},
		{name: "Malformed token", auth: "Bearer not-a-jwt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.auth != "" {
				r.Header.Set(authorizationHeader, tt.auth)
			}

			got, ok := GetExpiresAtFromRequest(r)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.True(t, tt.expected.Equal(got), "expected %v, got %v", tt.expected, got)
			}
		})
	}
}
