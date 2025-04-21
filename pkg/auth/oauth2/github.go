//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"net/http"
	"reflect"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

type GitHubAuthenticator struct {
	config Config
	state  string
	*baseOauth2Authenticator
}

type GitHubUserInfo struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// NewGitHubConfigs returns a new Config for GitHub.
func NewGitHubConfigs(clientId, clientSecret, redirectURL, redirectPath string) Config {
	c := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"user:email"}, // default and common scopes of GitHub
		Endpoint:     github.Endpoint,
	}

	if redirectPath == "" {
		redirectPath = "/"
	}

	config := Config{
		GoOAuth2Config: c,
		UserInfoURL:    githubUserInfoURL,
		RedirectPath:   redirectPath,
	}
	return config
}

func NewGitHubAuthenticator(config Config, lc log.Logger) *GitHubAuthenticator {
	// state should be a random string to protect against CSRF attacks
	state := uuid.NewString()
	baseOauth2Authenticator := newBaseOauth2Authenticator(lc)
	lc.Debugf("Initiating %s authenticator.", GitHub)
	return &GitHubAuthenticator{config: config, state: state, baseOauth2Authenticator: baseOauth2Authenticator}
}

// RequestAuth returns a http.HandlerFunc that redirects the user to the OAuth2 provider for authentication.
func (g *GitHubAuthenticator) RequestAuth() http.HandlerFunc {
	return g.requestAuth(g.config, g.state)
}

// Callback returns a http.HandlerFunc that exchanges the authorization code for an access token and fetches user info from the OAuth2 provider.
// The parameter is a function that takes the user info and returns the JWT token or an error.
func (g *GitHubAuthenticator) Callback(loginAndGetJWT func(userInfo any) (token *jwt.TokenDetails, err errors.Error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.callback(w, r, g.config, g.state, reflect.TypeOf(GitHubUserInfo{}), loginAndGetJWT)
	}
}

// GetTokenByUserID returns the oauth2 token by user ID
func (g *GitHubAuthenticator) GetTokenByUserID(userId string) (*oauth2.Token, errors.Error) {
	token, err := g.getTokenByUserID(g.config, userId)
	if err != nil {
		return nil, errors.BaseErrorWrapper(err)
	}

	return token, nil
}
