//
// Copyright (C) 2024 IOTech Ltd
//

package jwt

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
)

// GetTokenStringFromRequest gets the token string from the request header
func GetTokenStringFromRequest(r *http.Request) (string, errors.Error) {
	auth := r.Header.Get(authorizationHeader)
	if auth == "" {
		return "", errors.NewBaseError(errors.KindUnauthorized, authRequiredMsg, nil, nil)
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(auth, bearer))
	return tokenString, nil
}

// CreateToken creates a new token with the given name and expiration time, specified in hours from now with the default expiration time of 2 hours for access token and 7 days for refresh token
func CreateToken(name string, atExpiresFromNow *int64, reExpiresFromNow *int64) (*TokenDetails, errors.Error) {
	var err error
	td := &TokenDetails{}

	if atExpiresFromNow != nil {
		td.AtExpires = time.Now().Add(time.Hour * time.Duration(*atExpiresFromNow)).Unix()
	}
	if reExpiresFromNow != nil {
		td.RtExpires = time.Now().Add(time.Hour * time.Duration(*reExpiresFromNow)).Unix()
	}

	td.AccessId = uuid.New().String()
	td.AtExpires = time.Now().Add(time.Hour * 2).Unix() // default to 2 hours
	td.RefreshId = uuid.New().String()
	td.RtExpires = time.Now().Add(time.Hour * 24 * 7).Unix() // default to 7 days
	// Creating Access Token
	atClaims := jwt.MapClaims{}
	atClaims[issuer] = IOTechIssuer
	atClaims[authorized] = true
	atClaims[claimUsername] = name
	atClaims[claimAccessId] = td.AccessId
	atClaims[expiresAt] = td.AtExpires
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(secretKey))
	if err != nil {
		return nil, errors.NewBaseError(errors.KindServerError, failMsg, err, nil)
	}

	rtClaims := jwt.MapClaims{}
	rtClaims[claimUsername] = name
	rtClaims[claimRefreshId] = td.RefreshId
	atClaims[expiresAt] = td.RtExpires
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(refreshSecretKey))
	if err != nil {
		return nil, errors.NewBaseError(errors.KindServerError, failMsg, err, nil)
	}

	return td, nil
}

// ValidateToken validates the given token string and gets the accessId and username.
func ValidateToken(tokenString string, secretKey string) (string, string, errors.Error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header[algorithm])
		}

		return []byte(secretKey), nil
	})
	if err != nil {
		return "", "", errors.NewBaseError(errors.KindUnauthorized, invalidMsg, err, nil)
	}

	claim, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", errors.NewBaseError(errors.KindUnauthorized, invalidMsg, nil, nil)
	}

	refreshId, ok := claim[claimRefreshId].(string)
	if !ok {
		return "", "", errors.NewBaseError(errors.KindServerError, unexpectedMsg, nil, nil)
	}

	username, ok := claim[claimUsername].(string)
	if !ok {
		return "", "", errors.NewBaseError(errors.KindServerError, unexpectedMsg, nil, nil)
	}

	return refreshId, username, nil
}
