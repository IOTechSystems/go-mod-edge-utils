//
// Copyright (C) 2024 IOTech Ltd
//

package jwt

// Constants related to JWT
const (
	IOTechIssuer = "IOTech"

	algorithm        = "alg"
	authorized       = "authorized"
	claimAccessId    = "access_id"
	claimRefreshId   = "refresh_id"
	claimUsername    = "user_name"
	expiresAt        = "exp"
	issuer           = "iss"
	refreshSecretKey = "IOTechRefresh"
	secretKey        = "IOTechSystems"
)

// Constants related to HTTP headers
const (
	authorizationHeader = "Authorization"
	bearer              = "Bearer"
)

// Constants related to error messages
const (
	authRequiredMsg = "JWT Authentication required"
	failMsg         = "failed to sign and create token"
	unexpectedMsg   = "unexpected result parsing token"
	invalidMsg      = "invalid token"
)
