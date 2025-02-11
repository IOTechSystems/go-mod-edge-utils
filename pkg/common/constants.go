// Copyright (C) 2023-2025 IOTech Ltd

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

// constants relate to the url query parameters
const (
	CommaSeparator = ","
	End            = "end"
	Limit          = "limit"
	Labels         = "labels"
	Offset         = "offset"
	Since          = "since"
	Start          = "start"
	Tail           = "tail"
	Timestamps     = "timestamps"
	Until          = "until"
)
