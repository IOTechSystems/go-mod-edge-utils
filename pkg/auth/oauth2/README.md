# README #
The auth/oauth2 package is used by Go services to apply OAuth2 authentication.

## OAuth2 Providers ##
### authentik ###

[authentik](https://goauthentik.io/) is an open-source identity and access management solution that supports OAuth2. The authentik OAuth2 provider is used to authenticate users and authorize access to services.

To use the authentik OAuth2 provider, you first need to set up an [authentik server](https://docs.goauthentik.io/docs/installation/), create and [OAuth2 Provider](https://docs.goauthentik.io/docs/providers/oauth2/) and create an [Application](https://docs.goauthentik.io/docs/applications) through their UI.

The client ID, client secret are required to configure the authentik OAuth2 provider.

Following is an example of how to use the auth/oauth2 package to authenticate users using the authentik OAuth2 provider.
```go
package main

import (
	goOauth2 "golang.org/x/oauth2"
	"net/http"

	"github.com/labstack/echo/v4"
	
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/jwt"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/oauth2"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

const (
	clientID     = "Your client ID"
	clientSecret = "Your client secret"
	// The redirect URL should be the same as the callback URL in your application
	redirectURL  = "http://localhost:8080/callback"
	
	// The following URLs are the authentik OAuth2 provider URLs whose domain should be replaced with your authentik server domain
	authURL      = "http://localhost:9000/application/o/authorize/"
	tokenURL     = "http://localhost:9000/application/o/token/"
	userInfoURL  = "http://localhost:9000/application/o/userinfo/"
	redirectPath = "/"
)

func main() {
	e := echo.New()

	// Set up the OAuth2 configuration for authentik
	config := oauth2.NewAuthentikConfigs(clientID, clientSecret, authURL, tokenURL, redirectURL, userInfoURL, redirectPath)

	logger := log.InitLogger("main", log.InfoLog, nil)

	// Create the authentik OAuth2 authenticator
	oauth2Authenticator := oauth2.NewAuthentikAuthenticator(config, logger)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	// Set up the login and callback routes
	e.GET("/login", echo.WrapHandler(oauth2Authenticator.RequestAuth()))
	e.GET("/callback", echo.WrapHandler(oauth2Authenticator.Callback(handleUserInfo)))
	e.Logger.Fatal(e.Start(":8080"))
}

// handleUserInfo is a callback function that is called after the user is authenticated from the OAuth2 provider.
func handleUserInfo(userInfo any) (token *jwt.TokenDetails, err errors.Error) {
	userInfo, ok := userInfo.(oauth2.AuthentikUserInfo)
	if !ok {
		return nil, errors.NewBaseError(errors.KindServerError, "failed to cast user info to AuthentikUserInfo", nil, nil)
	}

	fakeToken := &jwt.TokenDetails{
		AccessToken:  "accesstoken",
		RefreshToken: "refreshtoken",
		AccessId:     "accessid",
		RefreshId:    "refreshid",
		AtExpires:    0,
		RtExpires:    0,
	}
	return fakeToken, nil
}
```
