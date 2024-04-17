//
// Copyright (C) 2024 IOTech Ltd
//

package jwt

type TokenDetails struct {
	AccessId     string
	AccessToken  string
	AtExpires    int64
	RefreshId    string
	RefreshToken string
	RtExpires    int64
}
