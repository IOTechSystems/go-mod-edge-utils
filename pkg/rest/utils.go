//
// Copyright (C) 2024 IOTech Ltd
//

package rest

import (
	"context"
	"net/http"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/handlers"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
)

func WriteDefaultHttpHeader(w http.ResponseWriter, ctx context.Context, statusCode int) {
	w.Header().Set(common.CorrelationID, handlers.FromContext(ctx))
	w.Header().Set(common.ContentType, common.ContentTypeJSON)
	w.WriteHeader(statusCode)
}
