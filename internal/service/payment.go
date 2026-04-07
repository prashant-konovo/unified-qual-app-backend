package service

import (
	"context"
	"database/sql"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// PaymentService encapsulates payment, honorarium, and reward logic.
type PaymentService struct {
	answerRepo   qs.AnswerRepository
	timeSlotRepo qs.TimeSlotRepository
	projectRepo  qs.ProjectRepository
	lambda       *integration.LambdaClient
	stepFn       *integration.StepFunctionsClient
	awsEnv       string
	icApiURL     string
}

// NewPaymentService creates a new PaymentService.
func NewPaymentService(answerRepo qs.AnswerRepository, timeSlotRepo qs.TimeSlotRepository, projectRepo qs.ProjectRepository, lambda *integration.LambdaClient, stepFn *integration.StepFunctionsClient, awsEnv, icApiURL string) *PaymentService {
	return &PaymentService{answerRepo: answerRepo, timeSlotRepo: timeSlotRepo, projectRepo: projectRepo, lambda: lambda, stepFn: stepFn, awsEnv: awsEnv, icApiURL: icApiURL}
}

// AnswerAvailable returns true if the QS answer repository is configured.
func (s *PaymentService) AnswerAvailable() bool { return s.answerRepo != nil }

// TimeSlotAvailable returns true if the QS timeslot repository is configured.
func (s *PaymentService) TimeSlotAvailable() bool { return s.timeSlotRepo != nil }

// ProjectAvailable returns true if the QS project repository is configured.
func (s *PaymentService) ProjectAvailable() bool { return s.projectRepo != nil }

func (s *PaymentService) LambdaConfigured() bool {
	return s.lambda != nil && s.lambda.Configured()
}

func (s *PaymentService) InvokeLambda(ctx context.Context, functionName string, payload []byte) ([]byte, int32, error) {
	return s.lambda.Invoke(ctx, functionName, payload)
}

func (s *PaymentService) FullLambdaName(baseName string) string {
	return s.lambda.FullLambdaName(baseName)
}

func (s *PaymentService) StepFnConfigured() bool {
	return s.stepFn != nil && s.stepFn.Configured()
}

func (s *PaymentService) StartStepFunctionExecution(ctx context.Context, stateMachineName string, input map[string]any) error {
	return s.stepFn.StartExecution(ctx, stateMachineName, input)
}

func (s *PaymentService) AWSEnvironment() string {
	return s.awsEnv
}

func (s *PaymentService) ICApiURL() string {
	return s.icApiURL
}

// --- LS payment methods (QsAnswerRepo) ---

func (s *PaymentService) CreatePaymentRecord(ctx context.Context, timeSlotID int64, amount int, paymentType, status string) (int64, error) {
	return s.answerRepo.CreatePaymentRecord(ctx, timeSlotID, amount, paymentType, status)
}

func (s *PaymentService) CreateCustomHonorarium(ctx context.Context, timeSlotID int64, amount int, reason string) (int64, error) {
	return s.answerRepo.CreateCustomHonorarium(ctx, timeSlotID, amount, reason)
}

func (s *PaymentService) CreateExternalPayment(ctx context.Context, timeSlotID int64, amount int, paymentType, status, externalRef string) (int64, error) {
	return s.answerRepo.CreateExternalPayment(ctx, timeSlotID, amount, paymentType, status, externalRef)
}

func (s *PaymentService) ListHonorariumReasons(ctx context.Context) ([]map[string]any, error) {
	return s.answerRepo.ListHonorariumReasons(ctx)
}

func (s *PaymentService) ListInterviewPaymentStatuses(ctx context.Context) ([]map[string]any, error) {
	return s.answerRepo.ListInterviewPaymentStatuses(ctx)
}

// --- MRA payment methods (QsTimeSlotRepo) ---

func (s *PaymentService) GetPaymentInfoByTimeSlotIdsMRA(ctx context.Context, timeSlotIDs []int64) ([]map[string]any, error) {
	return s.timeSlotRepo.GetPaymentInfoByTimeSlotIdsMRA(ctx, timeSlotIDs)
}

func (s *PaymentService) GetTimeSlotPaymentTypeListMRA(ctx context.Context) ([]map[string]any, error) {
	return s.timeSlotRepo.GetTimeSlotPaymentTypeListMRA(ctx)
}

func (s *PaymentService) GetUserByEmailMRA(ctx context.Context, email string) (int64, error) {
	return s.timeSlotRepo.GetUserByEmailMRA(ctx, email)
}

func (s *PaymentService) AddQSTimeSlotPaymentsMRA(ctx context.Context, payments []map[string]any) error {
	return s.timeSlotRepo.AddQSTimeSlotPaymentsMRA(ctx, payments)
}

func (s *PaymentService) AddExternalTimeSlotPaymentsMRA(ctx context.Context, payments []map[string]any) error {
	return s.timeSlotRepo.AddExternalTimeSlotPaymentsMRA(ctx, payments)
}

func (s *PaymentService) AddTimeSlotCustomHonorariumMRA(ctx context.Context, timeSlotID int64, oldValue, newValue float64, reasonID, createdBy int64) error {
	return s.timeSlotRepo.AddTimeSlotCustomHonorariumMRA(ctx, timeSlotID, oldValue, newValue, reasonID, createdBy)
}

func (s *PaymentService) GetExternalSurveyIdByTimeSlotIdMRA(ctx context.Context, timeSlotID int64) (string, error) {
	return s.timeSlotRepo.GetExternalSurveyIdByTimeSlotIdMRA(ctx, timeSlotID)
}

func (s *PaymentService) GetTimeSlotPaymentStatusListMRA(ctx context.Context) ([]map[string]any, error) {
	return s.timeSlotRepo.GetTimeSlotPaymentStatusListMRA(ctx)
}

// Transaction-based payment methods (tx created by handler, passed through)

func (s *PaymentService) GetPendingPaymentsMRA(ctx context.Context, tx *sql.Tx, timeSlotIDs []int64) ([]qs.PendingPaymentRecord, error) {
	return s.timeSlotRepo.GetPendingPaymentsMRA(ctx, tx, timeSlotIDs)
}

func (s *PaymentService) UpdateCompletedPaymentHistoryMRA(ctx context.Context, tx *sql.Tx, timeSlotIDs []int64, paymentHistoryIDs []int64) error {
	return s.timeSlotRepo.UpdateCompletedPaymentHistoryMRA(ctx, tx, timeSlotIDs, paymentHistoryIDs)
}

func (s *PaymentService) UpdateCanceledPaymentHistoryMRA(ctx context.Context, tx *sql.Tx, timeSlotIDs []int64, paymentHistoryIDs []int64) error {
	return s.timeSlotRepo.UpdateCanceledPaymentHistoryMRA(ctx, tx, timeSlotIDs, paymentHistoryIDs)
}

func (s *PaymentService) UpdateFailedPaymentHistoryMRA(ctx context.Context, timeSlotIDs []int64) error {
	return s.timeSlotRepo.UpdateFailedPaymentHistoryMRA(ctx, timeSlotIDs)
}

// BeginQSTx starts a database transaction on the QS database via TimeSlotRepo.
func (s *PaymentService) BeginQSTx(ctx context.Context) (*sql.Tx, error) {
	return s.timeSlotRepo.BeginTx(ctx)
}

// --- MRA honorarium methods (QsProjectRepo) ---

func (s *PaymentService) AddHonorariumAmountMRA(ctx context.Context, projectID, honorarium int64, currency, sessKey, extProjectID, extUserSurveyID, extUserID, extCreditOrderID, extCountryID string) error {
	return s.projectRepo.AddHonorariumAmountMRA(ctx, projectID, honorarium, currency, sessKey, extProjectID, extUserSurveyID, extUserID, extCreditOrderID, extCountryID)
}

func (s *PaymentService) GetHonoValueUpdateReasonListMRA(ctx context.Context) ([]map[string]any, error) {
	return s.projectRepo.GetHonoValueUpdateReasonListMRA(ctx)
}
