//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

type GoogleAuthenticator struct {
	config Config
	state  string
	*baseOauth2Authenticator
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
	HostedDomain  string `json:"hd"`
}

// NewGoogleConfigs returns a new Config for Google.
func NewGoogleConfigs(clientId, clientSecret, redirectURL, redirectPath string) Config {
	c := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "profile", "email"}, // default and common scopes of Google
		Endpoint:     google.Endpoint,
	}

	if redirectPath == "" {
		redirectPath = "/"
	}

	config := Config{
		GoOAuth2Config: c,
		UserInfoURL:    googleUserInfoURL,
		RedirectPath:   redirectPath,
	}
	return config
}

func NewGoogleAuthenticator(config Config, lc log.Logger) *GoogleAuthenticator {
	// state should be a random string to protect against CSRF attacks
	state := uuid.NewString()
	baseOauth2Authenticator := newBaseOauth2Authenticator(lc)
	lc.Debugf("Initiating %s authenticator.", Google)
	return &GoogleAuthenticator{config: config, state: state, baseOauth2Authenticator: baseOauth2Authenticator}
}

// RequestAuth returns a http.HandlerFunc that redirects the user to the OAuth2 provider for authentication.
func (g *GoogleAuthenticator) RequestAuth() http.HandlerFunc {
	return g.requestAuth(g.config, g.state)
}

// Callback returns a http.HandlerFunc that exchanges the authorization code for an access token and fetches user info from the OAuth2 provider.
// The parameter is a function that takes the user info and returns the JWT token or an error.
func (g *GoogleAuthenticator) Callback(loginAndGetJWT func(userInfo any) (token *jwt.TokenDetails, err errors.Error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.callback(w, r, g.config, g.state, reflect.TypeOf(GoogleUserInfo{}), loginAndGetJWT)
	}
}

// GetTokenByUserID returns the oauth2 token by user ID
func (g *GoogleAuthenticator) GetTokenByUserID(userId string) (*oauth2.Token, errors.Error) {
	token, err := g.getTokenByUserID(g.config, userId)
	if err != nil {
		return nil, errors.BaseErrorWrapper(err)
	}

	return token, nil
}

// Validate validates user info
func (u *GoogleUserInfo) Validate() error {
	if !u.VerifiedEmail {
		return fmt.Errorf("'%s' is not verified email", u.Email)
	}
	return nil
}
