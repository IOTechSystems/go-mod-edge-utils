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

// Constants related to HTTP headers
const (
	AccessTokenHeader  = "Access-Token"
	RefreshTokenHeader = "Refresh-Token"

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
