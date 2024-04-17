//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	httpUtils "github.com/IOTechSystems/go-mod-edge-utils/pkg/http"
)

type AuthentikAuthenticator struct {
	Config      *oauth2.Config
	UserInfoURL string
	state       string
}

type AuthentikUserInfo struct {
	ID                string   `json:"sub"`
	Email             string   `json:"email"`
	VerifiedEmail     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	GivenName         string   `json:"given_name"`
	PreferredUsername string   `json:"preferred_username"`
	Nickname          string   `json:"nickname"`
	Groups            []string `json:"groups"`
}

// NewAuthentikConfigs returns an oauth2.Config object with the given parameters.
func NewAuthentikConfigs(clientId string, clientSecret string, redirectURL string, endpoint oauth2.Endpoint) *oauth2.Config {
	config := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "profile", "email"}, // default and common scopes of Authentik
		Endpoint:     endpoint,
	}
	return config
}

// NewAuthentikAuthenticator creates a new Authenticator for Authentik.
func NewAuthentikAuthenticator(config *oauth2.Config, userInfoURL string) Authenticator {
	// state should be a random string to protect against CSRF attacks
	state := uuid.NewString()
	return &AuthentikAuthenticator{Config: config, UserInfoURL: userInfoURL, state: state}
}

// RequestAuth returns a http.HandlerFunc that redirects the user to the OAuth2 provider for authentication.
func (a *AuthentikAuthenticator) RequestAuth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := a.Config.AuthCodeURL(a.state, oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

// Callback returns a http.HandlerFunc that exchanges the authorization code for an access token and fetches user info from the OAuth2 provider.
func (a *AuthentikAuthenticator) Callback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get(codeParam)
		state := r.URL.Query().Get(stateParam)
		if state != a.state {
			http.Error(w, "invalid state. You may under CSRF attack.", http.StatusUnauthorized)
			return
		}

		token, err := a.Config.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to exchange token, err: %v", err), http.StatusInternalServerError)
			return
		}

		client := a.Config.Client(r.Context(), token)
		resp, err := client.Get(a.UserInfoURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch user info: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		userData, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("fail to read the response body: %v", err), http.StatusInternalServerError)
			return
		}

		userInfo := AuthentikUserInfo{}
		err = json.Unmarshal(userData, &userInfo)
		if err != nil {
			http.Error(w, fmt.Sprintf("fail to parse the response body: %v", err), http.StatusInternalServerError)
			return
		}

		err = userInfo.Validate()
		if err != nil {
			http.Error(w, fmt.Sprintf("user info validation failed: %v", err), http.StatusUnauthorized)
			return
		}
		httpUtils.WriteHttpHeader(w, r.Context(), http.StatusOK)
		_, err = w.Write(userData)
		if err != nil {
			http.Error(w, fmt.Sprintf("fail to write the response body: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// Validate validates user info
func (u *AuthentikUserInfo) Validate() error {
	if !u.VerifiedEmail {
		return fmt.Errorf("'%s' is not verified email", u.Email)
	}
	return nil
}
