//
// Copyright (C) 2024 IOTech Ltd
//

package jwt

// Constants related to JWT
const (
	IOTechIssuer = "IOTech"

	Algorithm      = "alg"
	Authorized     = "authorized"
	ClaimAccessId  = "access_id"
	ClaimRefreshId = "refresh_id"
	ClaimUsername  = "user_name"
	ExpiresAt      = "exp"
	Issuer         = "iss"
)

// Constants related to Cookie and HTTP headers
const (
	AccessTokenCookie = "IOTech_access_token"
	// This is not potential hardcoded credentials
	// nolint:gosec
	RefreshTokenCookie = "IOTech_refresh_token"

	authorizationHeader = "Authorization"
	bearer              = "Bearer"
)

// Constants related to error messages
const (
	authRequiredMsg = "JWT Authentication required"
	authRevokedMsg  = "authorization revoked"
	failMsg         = "failed to sign and create token"
	unexpectedMsg   = "unexpected result parsing token"
	invalidMsg      = "invalid token"
)
