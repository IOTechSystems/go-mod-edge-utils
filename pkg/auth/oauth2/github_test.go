//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

func getMockGithubConfigs() Config {
	config := NewGitHubConfigs(mockClientID, mockClientSecret, mockRedirectURL, mockRedirectPath)
	config.UserInfoURL = mockSeverURL + mockGithubUserInfoPath
	config.GoOAuth2Config.Endpoint.AuthURL = mockSeverURL + mockGithubAuthPath
	config.GoOAuth2Config.Endpoint.TokenURL = mockSeverURL + mockGithubTokenPath

	return config
}

func newGithubAuthenticator() Authenticator {
	logger := log.InitLogger(mockServiceName, log.InfoLog, nil)
	configs := getMockGithubConfigs()
	return NewGitHubAuthenticator(configs, logger)
}

func TestGithubRequestAuth(t *testing.T) {
	performRequestAuth(t, GitHub)
}

func TestGithubCallbackWithCorrectState(t *testing.T) {
	authenticator, state := performRequestAuth(t, GitHub)
	rr := performCallback(t, state, authenticator, GitHub)

	// Check if the response status code is http.StatusSeeOther (303)
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusSeeOther)
	}

	// Check if the Tokens are set as cookies
	foundAccessToken := false
	foundRefreshToken := false
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == jwt.AccessTokenCookie {
			foundAccessToken = true
		}
		if cookie.Name == jwt.RefreshTokenCookie {
			foundRefreshToken = true
		}
	}

	if !foundAccessToken || !foundRefreshToken {
		t.Errorf("handler did not set expected cookies. Access Token: %v, Refresh Token: %v", foundAccessToken, foundRefreshToken)
	}
}

func TestGithubCallbackWithIncorrectState(t *testing.T) {
	authenticator := newGithubAuthenticator()
	rr := performCallback(t, "", authenticator, GitHub)

	// Check if the response status code is http.StatusUnauthorized (401)
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}

	// Check if the response body contains the expected message
	expectedMsg := "invalid state. You may be under CSRF attack.\n"
	if body := rr.Body.String(); body != expectedMsg {
		t.Errorf("handler returned unexpected body: got %v want %v", body, expectedMsg)
	}
}

func TestGithubGetTokenByUserIDWithTokenNotFound(t *testing.T) {
	authenticator := newGithubAuthenticator()
	_, err := authenticator.GetTokenByUserID(strconv.FormatInt(mockUserNumId, 10))

	assert.ErrorIs(t, err, errors.NewBaseError(errors.KindEntityDoesNotExist, fmt.Sprintf("token not found for the user %v", mockUserNumId), nil, nil))
}

func TestGithubGetTokenByUserID(t *testing.T) {
	authenticator, state := performRequestAuth(t, GitHub)
	rr := performCallback(t, state, authenticator, GitHub)

	// Check if the response status code is http.StatusSeeOther (303)
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusSeeOther)
	}

	// Check if the Tokens are cached for the user
	token, err := authenticator.GetTokenByUserID(strconv.FormatInt(mockUserNumId, 10))

	assert.NoError(t, err, "should not get an error")
	assert.NotNil(t, token, "token should not be nil")
}
