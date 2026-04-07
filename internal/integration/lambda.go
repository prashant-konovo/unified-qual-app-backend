package integration

import (
	"context"
	"fmt"
	"log/slog"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Lambda Client (AWS SDK v2)
// ──────────────────────────────────────────────

// LambdaClient wraps AWS Lambda invocations.
type LambdaClient struct {
	region      string
	environment string
	client      *lambda.Client
}

func newLambdaClient(cfg config.AWSConfig) *LambdaClient {
	lc := &LambdaClient{region: cfg.Region, environment: cfg.Environment}
	if cfg.Region != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.Region))
		if err == nil {
			lc.client = lambda.NewFromConfig(awsCfg)
		} else {
			slog.Warn("lambda client init failed", "error", err)
		}
	}
	return lc
}

func (lc *LambdaClient) Configured() bool {
	return lc.client != nil && lc.environment != ""
}

// Invoke calls a Lambda function by name with a JSON payload and returns the response payload.
func (lc *LambdaClient) Invoke(ctx context.Context, functionName string, payload []byte) ([]byte, int32, error) {
	if !lc.Configured() {
		return nil, 0, fmt.Errorf("lambda client not configured")
	}

	out, err := lc.client.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: &functionName,
		Payload:      payload,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("lambda invoke %s: %w", functionName, err)
	}

	return out.Payload, out.StatusCode, nil
}

// FullLambdaName returns the environment-prefixed Lambda function name.
// Legacy mapping: "prd" → "production", "data-qa" → "qual-qa", else as-is.
func (lc *LambdaClient) FullLambdaName(baseName string) string {
	prefix := lc.environment
	switch prefix {
	case "prd":
		prefix = "production"
	case "data-qa":
		prefix = "qual-qa"
	}
	return prefix + "-" + baseName
}
