package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/peniakoff/weles/internal/app"
)

func main() {
	ctx := context.Background()
	deps, err := app.Build(ctx)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	adapter := httpadapter.NewV2(deps.Handler())
	lambda.Start(adapter.ProxyWithContext)
}
