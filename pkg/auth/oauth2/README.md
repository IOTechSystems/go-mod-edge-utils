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

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/auth/oauth2"
	"github.com/labstack/echo/v4"
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
)

func main() {
	e := echo.New()

	// Set up the OAuth2 configuration for authentik
	authEndpoint := goOauth2.Endpoint{
		AuthURL:  authURL,
		TokenURL: tokenURL,
	}
	config := oauth2.NewAuthentikConfigs(clientID, clientSecret, redirectURL, authEndpoint)

	// Create the authentik OAuth2 authenticator
	oauth2Authenticator := oauth2.NewAuthentikAuthenticator(config, userInfoURL)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	// Set up the login and callback routes
	e.GET("/login", echo.WrapHandler(oauth2Authenticator.RequestAuth()))
	e.GET("/callback", echo.WrapHandler(oauth2Authenticator.Callback()))
	e.Logger.Fatal(e.Start(":8080"))
}
```
