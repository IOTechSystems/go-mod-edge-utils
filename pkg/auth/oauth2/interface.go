//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"net/http"

	"golang.org/x/oauth2"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
)

// Authenticator is an interface for OAuth2 authenticators.
type Authenticator interface {
	// RequestAuth returns a http.HandlerFunc that redirects the user to the OAuth2 provider for authentication and gets the authorization code.
	RequestAuth() http.HandlerFunc
	// Callback returns a http.HandlerFunc that exchanges the authorization code for an access token and fetches user info from the OAuth2 provider.
	// The parameter is a function that takes the user info and returns the JWT token or an error.
	Callback(func(userInfo any) (token *jwt.TokenDetails, err errors.Error)) http.HandlerFunc
	// GetTokenByUserID returns the cache token by user ID.
	GetTokenByUserID(userId string) (*oauth2.Token, errors.Error)
}
