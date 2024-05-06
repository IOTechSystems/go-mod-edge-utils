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
func CreateToken(name, secretKey, refreshSecretKey string, atExpiresFromNow, reExpiresFromNow *int64) (*TokenDetails, errors.Error) {
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
	atClaims[Issuer] = IOTechIssuer
	atClaims[Authorized] = true
	atClaims[ClaimUsername] = name
	atClaims[ClaimAccessId] = td.AccessId
	atClaims[ExpiresAt] = td.AtExpires
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(secretKey))
	if err != nil {
		return nil, errors.NewBaseError(errors.KindServerError, failMsg, err, nil)
	}

	rtClaims := jwt.MapClaims{}
	rtClaims[Issuer] = IOTechIssuer
	rtClaims[ClaimUsername] = name
	rtClaims[ClaimRefreshId] = td.RefreshId
	rtClaims[ExpiresAt] = td.RtExpires
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(refreshSecretKey))
	if err != nil {
		return nil, errors.NewBaseError(errors.KindServerError, failMsg, err, nil)
	}

	return td, nil
}

// ValidateAccessToken validates the given access token string and gets the accessId and username.
func ValidateAccessToken(tokenString string, secretKey string) (string, string, errors.Error) {
	claim, err := validateToken(tokenString, secretKey)
	if err != nil {
		return "", "", errors.BaseErrorWrapper(err)
	}

	accessId, ok := claim[ClaimAccessId].(string)
	if !ok {
		return "", "", errors.NewBaseError(errors.KindServerError, unexpectedMsg, nil, nil)
	}

	username, ok := claim[ClaimUsername].(string)
	if !ok {
		return "", "", errors.NewBaseError(errors.KindServerError, unexpectedMsg, nil, nil)
	}

	return accessId, username, nil
}

// ValidateRefreshToken validates the given refresh token string and gets the refreshId and username.
func ValidateRefreshToken(tokenString string, refreshSecretKey string) (string, string, errors.Error) {
	claim, err := validateToken(tokenString, refreshSecretKey)
	if err != nil {
		return "", "", errors.BaseErrorWrapper(err)
	}

	refreshId, ok := claim[ClaimRefreshId].(string)
	if !ok {
		return "", "", errors.NewBaseError(errors.KindServerError, unexpectedMsg, nil, nil)
	}

	username, ok := claim[ClaimUsername].(string)
	if !ok {
		return "", "", errors.NewBaseError(errors.KindServerError, unexpectedMsg, nil, nil)
	}

	return refreshId, username, nil
}

// validateToken validates the given token string and gets claims map.
func validateToken(tokenString string, secretKey string) (jwt.MapClaims, errors.Error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header[Algorithm])
		}

		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, errors.NewBaseError(errors.KindUnauthorized, invalidMsg, err, nil)
	}

	claim, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.NewBaseError(errors.KindUnauthorized, invalidMsg, nil, nil)
	}

	sameIssuer := claim.VerifyIssuer(IOTechIssuer, true)
	if !sameIssuer {
		return nil, errors.NewBaseError(errors.KindUnauthorized, unexpectedMsg, nil, nil)
	}

	notExpired := claim.VerifyExpiresAt(jwt.TimeFunc().Unix(), true)
	if !notExpired {
		return nil, errors.NewBaseError(errors.KindUnauthorized, authRevokedMsg, nil, nil)
	}

	return claim, nil
}

// SetTokensToCookie sets the access token and refresh token to the response cookie
func SetTokensToCookie(w http.ResponseWriter, t *TokenDetails) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookie,
		Value:    t.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(t.AtExpires, 0),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    t.RefreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(t.RtExpires, 0),
	})
}

// GetTokensFromCookie gets the access token and refresh token from the request cookie
func GetTokensFromCookie(r *http.Request) (string, string) {
	accessCookie, err := r.Cookie(AccessTokenCookie)
	if err != nil {
		return "", ""
	}
	refreshCookie, err := r.Cookie(RefreshTokenCookie)
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(accessCookie.Value), strings.TrimSpace(refreshCookie.Value)
}

// RemoveTokensFromCookie removes the access token and refresh token from the response cookie
func RemoveTokensFromCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now(),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now(),
	})
}
