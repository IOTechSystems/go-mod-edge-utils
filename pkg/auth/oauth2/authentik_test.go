//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

var (
	mockSeverURL string
)

const (
	mockServiceName  = "testService"
	mockClientID     = "clientID"
	mockClientSecret = "clientSecret"
	mockAuthCode     = "authCode"
	mockCallbackPath = "/callback"
	mockAuthPath     = "/auth"
	mockRedirectURL  = "http://localhost:8080" + mockCallbackPath
)

func getMockConfigs() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     mockClientID,
		ClientSecret: mockClientSecret,
		RedirectURL:  mockRedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  mockSeverURL + mockAuthPath,
			TokenURL: mockSeverURL + mockTokenPath,
		},
	}
}

func newAuthentikAuthenticator() Authenticator {
	logger := log.InitLogger(mockServiceName, log.InfoLog, nil)
	configs := getMockConfigs()
	return NewAuthentikAuthenticator(configs, mockSeverURL+mockUserInfoPath, logger)
}

func TestMain(m *testing.M) {
	testMockServer := mockAuthenticServer()
	defer testMockServer.Close()

	URL, _ := url.Parse(testMockServer.URL)
	mockSeverURL = "http://" + URL.Host

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestRequestAuth(t *testing.T) {
	performRequestAuth(t)
}

func TestCallbackWithCorrectState(t *testing.T) {
	authenticator, state := performRequestAuth(t)
	rr := performCallback(t, state, authenticator)

	// Check if the response status code is http.StatusOK (200)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check if the response body contains the expected message
	if body := rr.Body.String(); body != mockUserInfo {
		t.Errorf("handler returned unexpected body: got %v want %v", body, mockUserInfo)
	}
}

func TestCallbackWithIncorrectState(t *testing.T) {
	authenticator := newAuthentikAuthenticator()
	rr := performCallback(t, "", authenticator)

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

func performRequestAuth(t *testing.T) (Authenticator, string) {
	authenticator := newAuthentikAuthenticator()
	// Create a testing HTTP request
	req, err := http.NewRequest("GET", mockAuthPath, nil)
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
	if "http://"+parsedURL.Host != mockSeverURL || parsedURL.Path != mockAuthPath {
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

func performCallback(t *testing.T, state string, authenticator Authenticator) *httptest.ResponseRecorder {
	if authenticator == nil {
		authenticator = newAuthentikAuthenticator()
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
	authenticator.Callback().ServeHTTP(rr, req)
	return rr
}
