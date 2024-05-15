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
	mockUserId       = "mockUserId"
	mockTokenPath    = "/token"
	mockUserInfoPath = "/userinfo"
	mockUserInfo     = `{"sub":"mockUserId","name":"test","email":"test@gmail.com","email_verified":true}`
)

func mockAuthenticServer() *httptest.Server {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, mockTokenPath) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, err := w.Write([]byte("access_token=90d64460d14870c08c81352a05dedd3465940a7c&token_type=bearer"))
			if err != nil {
				log.Printf("error writing response: %s", err.Error())
			}
		} else if strings.HasSuffix(r.URL.Path, mockUserInfoPath) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(mockUserInfo))
			if err != nil {
				log.Printf("error writing response: %s", err.Error())
			}
		}
	}))
	return mockServer
}
