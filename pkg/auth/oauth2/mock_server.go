//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
)

const (
	mockClientID     = "clientID"
	mockClientSecret = "clientSecret"
	mockUserId       = "mockUserId"
	mockUserNumId    = 123
	mockServiceName  = "testService"
	mockAuthCode     = "authCode"
	mockCallbackPath = "/callback"
	mockRedirectURL  = "http://localhost:8080" + mockCallbackPath
	mockRedirectPath = "/"

	mockAuthentikAuthPath = "/authentik/auth"
	// This is not potential hardcoded credentials
	// nolint:gosec
	mockAuthentikTokenPath    = "/authentik/token"
	mockAuthentikUserInfoPath = "/authentik/userinfo"
	mockAuthentikUserInfo     = `{"sub":"mockUserId","name":"test","email":"test@gmail.com","email_verified":true}`
	mockGoogleAuthPath        = "/google/auth"
	// This is not potential hardcoded credentials
	// nolint:gosec
	mockGoogleTokenPath    = "/google/token"
	mockGoogleUserInfoPath = "/google/userinfo"
	mockGoogleUserInfo     = `{"id":"mockUserId","name":"test","email":"test@gmail.com","verified_email":true}`
	mockGithubAuthPath     = "/github/auth"
	// This is not potential hardcoded credentials
	// nolint:gosec
	mockGithubTokenPath    = "/github/token"
	mockGithubUserInfoPath = "/github/userinfo"
	mockGithubUserInfo     = `{"id":123,"name":"test","email":"test@gmail.com"}`
)

func mockServer() *httptest.Server {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, mockAuthentikTokenPath) ||
			strings.HasSuffix(r.URL.Path, mockGoogleTokenPath) ||
			strings.HasSuffix(r.URL.Path, mockGithubTokenPath) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, err := w.Write([]byte("access_token=90d64460d14870c08c81352a05dedd3465940a7c&token_type=bearer"))
			if err != nil {
				log.Printf("error writing response: %s", err.Error())
			}
		} else if strings.HasSuffix(r.URL.Path, mockAuthentikUserInfoPath) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(mockAuthentikUserInfo))
			if err != nil {
				log.Printf("error writing response: %s", err.Error())
			}
		} else if strings.HasSuffix(r.URL.Path, mockGoogleUserInfoPath) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(mockGoogleUserInfo))
			if err != nil {
				log.Printf("error writing response: %s", err.Error())
			}
		} else if strings.HasSuffix(r.URL.Path, mockGithubUserInfoPath) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(mockGithubUserInfo))
			if err != nil {
				log.Printf("error writing response: %s", err.Error())
			}
		}
	}))
	return mockServer
}
