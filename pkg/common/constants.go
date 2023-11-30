// Copyright (C) 2023 IOTech Ltd

package common

const (
	CorrelationID   = "X-Correlation-ID"
	ContentType     = "Content-Type"
	ContentTypeJSON = "application/json"
	ContentTypeText = "text/plain"
)

const (
	ApiVersion = "v1"
	ApiBase    = "/api/v1"

	ApiConfigRoute  = ApiBase + "/config"
	ApiPingRoute    = ApiBase + "/ping"
	ApiVersionRoute = ApiBase + "/version"
	ApiSecretRoute  = ApiBase + "/secret"
)
