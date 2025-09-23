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
	"reflect"
	"strconv"
	"sync"

	"golang.org/x/oauth2"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

type baseOauth2Authenticator struct {
	// tokens is a map used to store the token details from the OAuth2 provider, the key is the user ID
	tokens map[string]*oauth2.Token
	mu     sync.RWMutex
	lc     log.Logger
}

func newBaseOauth2Authenticator(logger log.Logger) *baseOauth2Authenticator {
	return &baseOauth2Authenticator{
		tokens: make(map[string]*oauth2.Token),
		lc:     logger,
	}
}

func (b *baseOauth2Authenticator) requestAuth(config Config, state string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := config.GoOAuth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusFound)
	}
}

func (b *baseOauth2Authenticator) callback(w http.ResponseWriter, r *http.Request, config Config, state string, userInfoType reflect.Type, loginAndGetJWT func(userInfo any) (token *jwt.TokenDetails, err errors.Error)) {
	code := r.URL.Query().Get(codeParam)
	stateFormURL := r.URL.Query().Get(stateParam)
	if stateFormURL != state {
		b.lc.Error("State does not match, you may be under CSRF attack.")
		http.Error(w, "invalid state. You may be under CSRF attack.", http.StatusUnauthorized)
		return
	}

	token, err := config.GoOAuth2Config.Exchange(r.Context(), code)
	b.lc.Debugf("exchange authentication code %v for the access token", code)
	if err != nil {
		b.lc.Errorf("failed to exchange token, err: %v", err)
		http.Error(w, fmt.Sprintf("failed to exchange token, err: %v", err), http.StatusInternalServerError)
		return
	}

	client := config.GoOAuth2Config.Client(r.Context(), token)
	resp, err := client.Get(config.UserInfoURL)
	b.lc.Debugf("fetching user info from %v", config.UserInfoURL)
	if err != nil {
		b.lc.Errorf("failed to fetch user info: %v", err)
		http.Error(w, fmt.Sprintf("failed to fetch user info: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		b.lc.Errorf("failed to read the response body: %v", err)
		http.Error(w, fmt.Sprintf("fail to read the response body: %v", err), http.StatusInternalServerError)
		return
	}

	userInfoAny := reflect.New(userInfoType).Interface()
	err = json.Unmarshal(userData, &userInfoAny)
	if err != nil {
		b.lc.Errorf("failed to parse the response body: %v", err)
		http.Error(w, fmt.Sprintf("fail to parse the response body: %v", err), http.StatusInternalServerError)
		return
	}

	// Optional: Validate userInfo if it has a Validate method
	validateMethod := reflect.ValueOf(userInfoAny).MethodByName("Validate")
	if validateMethod.IsValid() {
		result := validateMethod.Call(nil)
		if len(result) > 0 && !result[0].IsNil() {
			err = result[0].Interface().(error)
			b.lc.Errorf("user info validation failed: %v", err)
			http.Error(w, fmt.Sprintf("user info validation failed: %v", err), http.StatusUnauthorized)
			return
		}
	}

	var userId string
	if userInfo, ok := userInfoAny.(*AuthentikUserInfo); ok {
		// Use the 'sub' field from authentik as the user ID
		userInfo.ID = userInfo.Sub
		userId = userInfo.ID
		userInfoAny = userInfo
	}
	if userInfo, ok := userInfoAny.(*GoogleUserInfo); ok {
		userId = userInfo.ID
	}
	if userInfo, ok := userInfoAny.(*GitHubUserInfo); ok {
		// GitHub's user ID is int64 type
		userId = strconv.FormatInt(userInfo.ID, 10)
		userInfoAny = userInfo
	}

	// Store the token details in the map
	b.mu.Lock()
	b.tokens[userId] = token
	b.mu.Unlock()

	tokenDetails, err := loginAndGetJWT(userInfoAny)
	if err != nil {
		b.lc.Errorf("failed to log in: %v", err)
		http.Error(w, fmt.Sprintf("failed to log in: %v", err), http.StatusInternalServerError)
		return
	}

	// Set the tokens to cookie and redirect to the redirect path
	jwt.SetTokensToCookie(w, tokenDetails)
	http.Redirect(w, r, config.RedirectPath, http.StatusSeeOther)
}

func (b *baseOauth2Authenticator) getTokenByUserID(config Config, userId string) (*oauth2.Token, errors.Error) {
	b.mu.RLock()
	token, ok := b.tokens[userId]
	b.mu.RUnlock()
	if !ok || token == nil {
		return nil, errors.NewBaseError(errors.KindEntityDoesNotExist, fmt.Sprintf("token not found for the user %s", userId), nil)
	}

	var err errors.Error
	if !token.Valid() {
		b.lc.Debug("Token is invalid or expired, try to refresh it.")
		// Try to refresh the token
		token, err = b.refreshOAuth2Token(config, userId, token)
		if err != nil {
			return nil, errors.BaseErrorWrapper(err)
		}
	}

	return token, nil
}

func (b *baseOauth2Authenticator) refreshOAuth2Token(config Config, userId string, token *oauth2.Token) (*oauth2.Token, errors.Error) {
	newToken, err := config.GoOAuth2Config.Exchange(context.Background(), token.RefreshToken)
	b.lc.Debug("exchange refresh token for the new token")
	if err != nil {
		b.lc.Errorf("failed to exchange token, err: %v", err)
		return nil, errors.NewBaseError(errors.KindServerError, "failed to exchange token", err)
	}

	// Store the new token
	b.mu.Lock()
	b.tokens[userId] = newToken
	b.mu.Unlock()

	return newToken, nil
}
