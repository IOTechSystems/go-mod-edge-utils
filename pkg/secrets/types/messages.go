//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package types

// TokenMetadata has introspection data about a token and is the "data" sub-structure for token lookup,
// i.e. TokenLookupResponse, and token self-lookup
type TokenMetadata struct {
	Accessor   string   `json:"accessor"`
	ExpireTime string   `json:"expire_time"`
	Path       string   `json:"path"`
	Policies   []string `json:"policies"`
	Period     int      `json:"period"` // in seconds
	Renewable  bool     `json:"renewable"`
	Ttl        int      `json:"ttl"` // in seconds
}
