//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

func getMockGoogleConfigs() Config {
	config := NewGoogleConfigs(mockClientID, mockClientSecret, mockRedirectURL, mockRedirectPath)
	config.UserInfoURL = mockSeverURL + mockGoogleUserInfoPath
	config.GoOAuth2Config.Endpoint.AuthURL = mockSeverURL + mockGoogleAuthPath
	config.GoOAuth2Config.Endpoint.TokenURL = mockSeverURL + mockGoogleTokenPath

	return config
}

func newGoogleAuthenticator() Authenticator {
	logger := log.InitLogger(mockServiceName, log.InfoLog, nil)
	configs := getMockGoogleConfigs()
	return NewGoogleAuthenticator(configs, logger)
}

func TestGoogleRequestAuth(t *testing.T) {
	performRequestAuth(t, mockGoogleProvider)
}

func TestGoogleCallbackWithCorrectState(t *testing.T) {
	authenticator, state := performRequestAuth(t, mockGoogleProvider)
	rr := performCallback(t, state, authenticator, mockGoogleProvider)

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

func TestGoogleCallbackWithIncorrectState(t *testing.T) {
	authenticator := newGoogleAuthenticator()
	rr := performCallback(t, "", authenticator, mockGoogleProvider)

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

func TestGoogleGetTokenByUserIDWithTokenNotFound(t *testing.T) {
	authenticator := newGoogleAuthenticator()
	_, err := authenticator.GetTokenByUserID(mockUserId)

	assert.ErrorIs(t, err, errors.NewBaseError(errors.KindEntityDoesNotExist, fmt.Sprintf("token not found for the user %s", mockUserId), nil, nil))
}

func TestGoogleGetTokenByUserID(t *testing.T) {
	authenticator, state := performRequestAuth(t, mockGoogleProvider)
	rr := performCallback(t, state, authenticator, mockGoogleProvider)

	// Check if the response status code is http.StatusSeeOther (303)
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusSeeOther)
	}

	// Check if the Tokens are cached for the user
	token, err := authenticator.GetTokenByUserID(mockUserId)

	assert.NoError(t, err, "should not get an error")
	assert.NotNil(t, token, "token should not be nil")
}
