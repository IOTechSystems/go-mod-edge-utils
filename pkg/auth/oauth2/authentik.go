//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"net/http"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

type AuthentikAuthenticator struct {
	config Config
	state  string
	*baseOauth2Authenticator
}

type AuthentikUserInfo struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	VerifiedEmail     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	GivenName         string   `json:"given_name"`
	PreferredUsername string   `json:"preferred_username"`
	Nickname          string   `json:"nickname"`
	Groups            []string `json:"groups"`

	// Custom fields of a more common name for the user ID
	ID string `json:"id"`
}

// NewAuthentikConfigs returns a new Config for authentik.
func NewAuthentikConfigs(clientId, clientSecret, authURL, tokenURL, redirectURL, userInfoURL, redirectPath string) Config {
	c := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "profile", "email"}, // default and common scopes of authentik
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}

	if redirectPath == "" {
		redirectPath = "/"
	}

	config := Config{
		GoOAuth2Config: c,
		UserInfoURL:    userInfoURL,
		RedirectPath:   redirectPath,
	}
	return config
}

// NewAuthentikAuthenticator creates a new Authenticator for authentik.
func NewAuthentikAuthenticator(config Config, lc log.Logger) Authenticator {
	// state should be a random string to protect against CSRF attacks
	state := uuid.NewString()
	baseOauth2Authenticator := newBaseOauth2Authenticator(lc)
	lc.Debugf("Initiating %s authenticator.", Authentik)
	return &AuthentikAuthenticator{config: config, state: state, baseOauth2Authenticator: baseOauth2Authenticator}
}

// RequestAuth returns a http.HandlerFunc that redirects the user to the OAuth2 provider for authentication.
func (a *AuthentikAuthenticator) RequestAuth() http.HandlerFunc {
	return a.requestAuth(a.config, a.state)
}

// Callback returns a http.HandlerFunc that exchanges the authorization code for an access token and fetches user info from the OAuth2 provider.
// The parameter is a function that takes the user info and returns the JWT token or an error.
func (a *AuthentikAuthenticator) Callback(loginAndGetJWT func(userInfo any) (token *jwt.TokenDetails, err errors.Error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.callback(w, r, a.config, a.state, reflect.TypeOf(AuthentikUserInfo{}), loginAndGetJWT)
	}
}

// GetTokenByUserID returns the oauth2 token by user ID
func (a *AuthentikAuthenticator) GetTokenByUserID(userId string) (*oauth2.Token, errors.Error) {
	token, err := a.getTokenByUserID(a.config, userId)
	if err != nil {
		return nil, errors.BaseErrorWrapper(err)
	}

	return token, nil
}

// Validate validates user info
func (u *AuthentikUserInfo) Validate() error {
	if !u.VerifiedEmail {
		return fmt.Errorf("'%s' is not verified email", u.Email)
	}
	return nil
}
