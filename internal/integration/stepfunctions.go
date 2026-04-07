package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Step Functions Client (AWS SDK v2)
// ──────────────────────────────────────────────

// StepFunctionsClient wraps AWS Step Functions invocations.
type StepFunctionsClient struct {
	region      string
	account     string
	environment string
	client      *sfn.Client
}

func newStepFunctionsClient(cfg config.AWSConfig) *StepFunctionsClient {
	sc := &StepFunctionsClient{
		region: cfg.Region, account: cfg.Account, environment: cfg.Environment,
	}
	if cfg.Region != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.Region))
		if err == nil {
			sc.client = sfn.NewFromConfig(awsCfg)
		} else {
			slog.Warn("step functions client init failed", "error", err)
		}
	}
	return sc
}

func (sc *StepFunctionsClient) Configured() bool {
	return sc.client != nil && sc.account != "" && sc.environment != ""
}

// StartExecution starts a Step Function execution with the given state machine name and input payload.
func (sc *StepFunctionsClient) StartExecution(ctx context.Context, stateMachineName string, input map[string]any) error {
	if !sc.Configured() {
		return fmt.Errorf("step functions client not configured")
	}

	arn := fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s-%s",
		sc.region, sc.account, sc.environment, stateMachineName)

	payload := map[string]any{"payload": input}
	inputJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal step function input: %w", err)
	}
	inputStr := string(inputJSON)

	_, err = sc.client.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: &arn,
		Input:           &inputStr,
	})
	if err != nil {
		return fmt.Errorf("start step function %s: %w", stateMachineName, err)
	}

	slog.Info("step function started", "arn", arn)
	return nil
}
