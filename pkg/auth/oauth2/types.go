//
// Copyright (C) 2024 IOTech Ltd
//

package oauth2

import "golang.org/x/oauth2"

type Config struct {
	GoOAuth2Config *oauth2.Config
	UserInfoURL    string
	RedirectPath   string // RedirectPath is the path that the user will be redirected to after login
}
