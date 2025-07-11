//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

var (
	mockSeverURL string
)

func getMockAuthentikConfigs() Config {
	return Config{
		GoOAuth2Config: &oauth2.Config{
			ClientID:     mockClientID,
			ClientSecret: mockClientSecret,
			RedirectURL:  mockRedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  mockSeverURL + mockAuthentikAuthPath,
				TokenURL: mockSeverURL + mockAuthentikTokenPath,
			},
		},
		UserInfoURL:  mockSeverURL + mockAuthentikUserInfoPath,
		RedirectPath: mockRedirectPath,
	}
}

func newAuthentikAuthenticator() Authenticator {
	logger := log.InitLogger(mockServiceName, log.InfoLog, nil)
	configs := getMockAuthentikConfigs()
	return NewAuthentikAuthenticator(configs, logger)
}

func TestMain(m *testing.M) {
	testMockServer := mockServer()
	defer testMockServer.Close()

	URL, _ := url.Parse(testMockServer.URL)
	mockSeverURL = "http://" + URL.Host

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestRequestAuth(t *testing.T) {
	performRequestAuth(t, Authentik)
}

func TestCallbackWithCorrectState(t *testing.T) {
	authenticator, state := performRequestAuth(t, Authentik)
	rr := performCallback(t, state, authenticator, Authentik)

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

func TestCallbackWithIncorrectState(t *testing.T) {
	authenticator := newAuthentikAuthenticator()
	rr := performCallback(t, "", authenticator, Authentik)

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

func TestGetTokenByUserIDWithTokenNotFound(t *testing.T) {
	authenticator := newAuthentikAuthenticator()
	_, err := authenticator.GetTokenByUserID(mockUserId)

	assert.ErrorIs(t, err, errors.NewBaseError(errors.KindEntityDoesNotExist, fmt.Sprintf("token not found for the user %s", mockUserId), nil, nil))
}

func TestGetTokenByUserID(t *testing.T) {
	authenticator, state := performRequestAuth(t, Authentik)
	rr := performCallback(t, state, authenticator, Authentik)

	// Check if the response status code is http.StatusSeeOther (303)
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusSeeOther)
	}

	// Check if the Tokens are cached for the user
	token, err := authenticator.GetTokenByUserID(mockUserId)

	assert.NoError(t, err, "should not get an error")
	assert.NotNil(t, token, "token should not be nil")
}

func performRequestAuth(t *testing.T, provider Provider) (Authenticator, string) {
	var (
		authenticator Authenticator
		authPath      string
	)
	switch provider {
	case Authentik:
		authenticator = newAuthentikAuthenticator()
		authPath = mockAuthentikAuthPath
	case Google:
		authenticator = newGoogleAuthenticator()
		authPath = mockGoogleAuthPath
	case GitHub:
		authenticator = newGithubAuthenticator()
		authPath = mockGithubAuthPath
	default:
		t.Fatal("invalid provider")
	}
	// Create a testing HTTP request
	req, err := http.NewRequest("GET", authPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Serve the RequestAuth HTTP request
	authenticator.RequestAuth().ServeHTTP(rr, req)

	// Check if the response status code is http.StatusFound (302)
	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusFound)
	}

	// Check if the Location header is set
	location := rr.Header().Get("Location")
	if location == "" {
		t.Errorf("handler did not set Location header")
	}

	// Parse the location URL
	parsedURL, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}

	// Check if the domain, port, and path are correct
	if "http://"+parsedURL.Host != mockSeverURL || parsedURL.Path != authPath {
		t.Errorf("handler returned unexpected location URL: got %v want %s://%s:%s%s", location, parsedURL.Scheme, parsedURL.Hostname(), parsedURL.Port(), parsedURL.Path)
	}

	// Check if the state parameter exists
	queryParams := parsedURL.Query()
	stateParam := queryParams.Get("state")
	if stateParam == "" {
		t.Error("handler did not include state parameter in the URL")
	}

	return authenticator, stateParam
}

func performCallback(t *testing.T, state string, authenticator Authenticator, provider Provider) *httptest.ResponseRecorder {
	if authenticator == nil {
		switch provider {
		case Authentik:
			authenticator = newAuthentikAuthenticator()
		case Google:
			authenticator = newGoogleAuthenticator()
		case GitHub:
			authenticator = newGithubAuthenticator()
		default:
			t.Fatal("invalid provider")
		}
	}

	// Create a testing HTTP request
	mockURL := mockCallbackPath
	params := url.Values{}
	params.Add("code", mockAuthCode)
	if state != "" {
		params.Add("state", state)
		mockURL = mockCallbackPath + "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", mockURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Serve the Callback HTTP request
	authenticator.Callback(func(userInfo any) (token *jwt.TokenDetails, err errors.Error) {
		return mockHandleUserInfo(userInfo, provider)
	}).ServeHTTP(rr, req)
	return rr
}

func mockHandleUserInfo(userInfo any, provider Provider) (token *jwt.TokenDetails, err errors.Error) {
	switch provider {
	case Authentik:
		_, ok := userInfo.(*AuthentikUserInfo)
		if !ok {
			return nil, errors.NewBaseError(errors.KindServerError, "failed to cast user info to AuthentikUserInfo", nil, nil)
		}
	case Google:
		_, ok := userInfo.(*GoogleUserInfo)
		if !ok {
			return nil, errors.NewBaseError(errors.KindServerError, "failed to cast user info to GoogleUserInfo", nil, nil)
		}
	case GitHub:
		_, ok := userInfo.(*GitHubUserInfo)
		if !ok {
			return nil, errors.NewBaseError(errors.KindServerError, "failed to cast user info to GitHubUserInfo", nil, nil)
		}
	}

	fakeToken := &jwt.TokenDetails{
		AccessToken:  "accesstoken",
		RefreshToken: "refreshtoken",
		AccessId:     "1123",
		RefreshId:    "123",
		AtExpires:    123,
		RtExpires:    123,
	}
	return fakeToken, nil
}
