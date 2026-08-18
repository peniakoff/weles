package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/peniakoff/weles/internal/app"
	"github.com/peniakoff/weles/internal/httpapi"
)

func main() {
	ctx := context.Background()
	deps, err := app.Build(ctx)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	handler := httpapi.StripStagePrefix(deps.Env.APIStage, deps.Handler())
	adapter := httpadapter.NewV2(handler)
	lambda.Start(adapter.ProxyWithContext)
}
