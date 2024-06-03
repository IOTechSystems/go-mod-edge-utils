//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

type GoogleAuthenticator struct {
	config Config
	// tokens is a map used to store the token details from the OAuth2 provider, the key is the user ID
	tokens map[string]*oauth2.Token
	mu     sync.RWMutex
	state  string
	lc     log.Logger
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
	lc.Debug("Initiating Google authenticator.")
	return &GoogleAuthenticator{
		config: config,
		tokens: make(map[string]*oauth2.Token),
		state:  state,
		lc:     lc,
	}
}

// RequestAuth returns a http.HandlerFunc that redirects the user to the OAuth2 provider for authentication.
func (g *GoogleAuthenticator) RequestAuth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := g.config.GoOAuth2Config.AuthCodeURL(g.state, oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

// Callback returns a http.HandlerFunc that exchanges the authorization code for an access token and fetches user info from the OAuth2 provider.
// The parameter is a function that takes the user info and returns the JWT token or an error.
func (g *GoogleAuthenticator) Callback(loginAndGetJWT func(userInfo any) (token *jwt.TokenDetails, err errors.Error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get(codeParam)
		state := r.URL.Query().Get(stateParam)
		if state != g.state {
			g.lc.Error("State does not match, you may be under CSRF attack.")
			http.Error(w, "invalid state. You may be under CSRF attack.", http.StatusUnauthorized)
			return
		}

		token, err := g.config.GoOAuth2Config.Exchange(r.Context(), code)
		g.lc.Debugf("exchange authentication code %v for the access token", code)
		if err != nil {
			g.lc.Errorf("failed to exchange token, err: %v", err)
			http.Error(w, fmt.Sprintf("failed to exchange token, err: %v", err), http.StatusInternalServerError)
			return
		}

		client := g.config.GoOAuth2Config.Client(r.Context(), token)
		resp, err := client.Get(g.config.UserInfoURL)
		g.lc.Debugf("fetching user info from %v", g.config.UserInfoURL)
		if err != nil {
			g.lc.Errorf("failed to fetch user info: %v", err)
			http.Error(w, fmt.Sprintf("failed to fetch user info: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		userData, err := io.ReadAll(resp.Body)
		if err != nil {
			g.lc.Errorf("failed to read the response body: %v", err)
			http.Error(w, fmt.Sprintf("fail to read the response body: %v", err), http.StatusInternalServerError)
			return
		}

		userInfo := GoogleUserInfo{}
		err = json.Unmarshal(userData, &userInfo)
		if err != nil {
			g.lc.Errorf("failed to parse the response body: %v", err)
			http.Error(w, fmt.Sprintf("fail to parse the response body: %v", err), http.StatusInternalServerError)
			return
		}

		err = userInfo.Validate()
		if err != nil {
			g.lc.Errorf("user info validation failed: %v", err)
			http.Error(w, fmt.Sprintf("user info validation failed: %v", err), http.StatusUnauthorized)
			return
		}

		// Store the token details in the map
		g.mu.Lock()
		g.tokens[userInfo.ID] = token
		g.mu.Unlock()

		tokenDetails, err := loginAndGetJWT(userInfo)
		if err != nil {
			g.lc.Errorf("failed to log in: %v", err)
			http.Error(w, fmt.Sprintf("failed to log in: %v", err), http.StatusInternalServerError)
			return
		}

		// Set the tokens to cookie and redirect to the redirect path
		jwt.SetTokensToCookie(w, tokenDetails)
		http.Redirect(w, r, g.config.RedirectPath, http.StatusSeeOther)
	}
}

// GetTokenByUserID returns the oauth2 token by user ID
func (g *GoogleAuthenticator) GetTokenByUserID(userId string) (*oauth2.Token, errors.Error) {
	g.mu.RLock()
	token, ok := g.tokens[userId]
	g.mu.RUnlock()
	if !ok || token == nil {
		return nil, errors.NewBaseError(errors.KindEntityDoesNotExist, fmt.Sprintf("token not found for the user %s", userId), nil, nil)
	}

	var err errors.Error
	if !token.Valid() {
		g.lc.Debug("Token is invalid or expired, try to refresh it.")
		// Try to refresh the token
		token, err = g.refreshToken(userId, token)
		if err != nil {
			return nil, errors.BaseErrorWrapper(err)
		}
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

// refreshToken refreshes the given token
func (g *GoogleAuthenticator) refreshToken(userId string, token *oauth2.Token) (*oauth2.Token, errors.Error) {
	if token == nil {
		return nil, nil
	}

	newToken, err := g.config.GoOAuth2Config.Exchange(context.Background(), token.RefreshToken)
	g.lc.Debug("exchange refresh token for the new token")
	if err != nil {
		g.lc.Errorf("failed to exchange token, err: %v", err)
		return nil, errors.NewBaseError(errors.KindServerError, "failed to exchange token", err, nil)
	}

	// Store the new token
	g.mu.Lock()
	g.tokens[userId] = newToken
	g.mu.Unlock()

	return newToken, nil
}
