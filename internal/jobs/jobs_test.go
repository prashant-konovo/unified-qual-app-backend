package jobs

import (
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	mocks "github.com/InCrowd/unified-qual-api/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testScheduler(jobsRepo *mocks.MockIrisJobsRepository) *Scheduler {
	return &Scheduler{
		deps: &JobDeps{
			JobsRepo: jobsRepo,
			Services: &integration.ServiceClients{},
		},
		cfg:    DefaultJobsConfig(),
		logger: slog.Default(),
	}
}

// ── IssueQualHonorarium ──────────────────────────────────────

func TestIssueQualHonorarium_PaysEligible(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	slots := []iris.PayableTimeSlot{{
		TimeSlotID:         100,
		UserID:             1,
		UserSurveyID:       10,
		SurveyID:           5,
		StartTime:          time.Now().Add(-48 * time.Hour),
		PromisedHonorarium: 50,
		UserEmail:          "user@test.com",
		UserTypeID:         1,
		MarketRewards:      true,
		SFProjectID:        sql.NullString{Valid: false},
		BrandTypeID:        2,
	}}

	repo.On("GetPayableTimeSlots", mock.Anything).Return(slots, nil)
	repo.On("GrantCredit", mock.Anything, mock.MatchedBy(func(p iris.CreditParams) bool {
		return p.UserID == 1 && p.Amount == 50 && p.ReasonID == 7
	})).Return(int64(999), nil)
	repo.On("MarkHonorariumPaid", mock.Anything, int64(100), int64(999)).Return(nil)

	s.issueQualHonorarium()

	repo.AssertExpectations(t)
	repo.AssertCalled(t, "GrantCredit", mock.Anything, mock.Anything)
	repo.AssertCalled(t, "MarkHonorariumPaid", mock.Anything, int64(100), int64(999))
}

func TestIssueQualHonorarium_SkipsIneligible(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	// userTypeID=2 → ineligible
	slots := []iris.PayableTimeSlot{{
		TimeSlotID:         200,
		UserID:             2,
		UserTypeID:         2,
		MarketRewards:      true,
		PromisedHonorarium: 50,
	}}

	repo.On("GetPayableTimeSlots", mock.Anything).Return(slots, nil)
	repo.On("MarkHonorariumPaid", mock.Anything, int64(200), int64(0)).Return(nil)

	s.issueQualHonorarium()

	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "GrantCredit", mock.Anything, mock.Anything)
}

func TestIssueQualHonorarium_NilRepo(t *testing.T) {
	s := &Scheduler{
		deps:   &JobDeps{Services: &integration.ServiceClients{}},
		cfg:    DefaultJobsConfig(),
		logger: slog.Default(),
	}
	// Should not panic
	s.issueQualHonorarium()
}

func TestIssueQualHonorarium_ContinuesOnError(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	slots := []iris.PayableTimeSlot{
		{TimeSlotID: 1, UserID: 1, UserTypeID: 1, MarketRewards: true, PromisedHonorarium: 10, StartTime: time.Now()},
		{TimeSlotID: 2, UserID: 2, UserTypeID: 1, MarketRewards: true, PromisedHonorarium: 20, StartTime: time.Now()},
	}

	repo.On("GetPayableTimeSlots", mock.Anything).Return(slots, nil)
	// First slot: GrantCredit fails
	repo.On("GrantCredit", mock.Anything, mock.MatchedBy(func(p iris.CreditParams) bool {
		return p.UserID == 1
	})).Return(int64(0), assert.AnError)
	// Second slot: succeeds
	repo.On("GrantCredit", mock.Anything, mock.MatchedBy(func(p iris.CreditParams) bool {
		return p.UserID == 2
	})).Return(int64(888), nil)
	repo.On("MarkHonorariumPaid", mock.Anything, int64(2), int64(888)).Return(nil)

	s.issueQualHonorarium()

	// Second slot should still be processed despite first failing
	repo.AssertCalled(t, "MarkHonorariumPaid", mock.Anything, int64(2), int64(888))
}

// ── CloseSurveyJob ──────────────────────────────────────────

func TestCloseSurveyJob_ClosesSurvey(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	surveys := []iris.SurveyToClose{{
		SurveyID:          10,
		SubscriptionID:    1,
		SFProjectStatus:   "Complete",
		SFProjectID:       "SF-001",
		NeedsToBeReviewed: false,
		IsInReview:        false,
	}}

	repo.On("GetFeatureFlag", mock.Anything, "enableCloseSurveyJob").Return(int64(1), nil)
	repo.On("GetSurveysToClose", mock.Anything).Return(surveys, nil)
	repo.On("CloseSurveyByJob", mock.Anything, int64(10)).Return(nil)
	repo.On("GetSurveyCrowdTypes", mock.Anything, int64(10)).Return([]int64{1, 2}, nil)
	repo.On("LogActivity", mock.Anything, mock.Anything).Return(nil)

	s.closeSurveyJob()

	repo.AssertExpectations(t)
	repo.AssertCalled(t, "CloseSurveyByJob", mock.Anything, int64(10))
}

func TestCloseSurveyJob_SetsInReview(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	surveys := []iris.SurveyToClose{{
		SurveyID:          20,
		SubscriptionID:    1,
		SFProjectStatus:   "Post Fieldwork",
		SFProjectID:       "SF-002",
		NeedsToBeReviewed: true,
		IsInReview:        false,
	}}

	repo.On("GetFeatureFlag", mock.Anything, "enableCloseSurveyJob").Return(int64(1), nil)
	repo.On("GetSurveysToClose", mock.Anything).Return(surveys, nil)
	repo.On("SetSurveyToInReview", mock.Anything, int64(20)).Return(nil)
	repo.On("GetSurveyCrowdTypes", mock.Anything, int64(20)).Return([]int64{}, nil)
	repo.On("LogActivity", mock.Anything, mock.Anything).Return(nil)

	s.closeSurveyJob()

	repo.AssertExpectations(t)
	repo.AssertCalled(t, "SetSurveyToInReview", mock.Anything, int64(20))
	repo.AssertNotCalled(t, "CloseSurveyByJob", mock.Anything, mock.Anything)
}

func TestCloseSurveyJob_DisabledByFlag(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	repo.On("GetFeatureFlag", mock.Anything, "enableCloseSurveyJob").Return(int64(0), nil)

	s.closeSurveyJob()

	repo.AssertNotCalled(t, "GetSurveysToClose", mock.Anything)
}

// ── Reminders ──────────────────────────────────────────────

func TestRemindDayBefore_NilRepo(t *testing.T) {
	s := &Scheduler{
		deps:   &JobDeps{Services: &integration.ServiceClients{}},
		cfg:    DefaultJobsConfig(),
		logger: slog.Default(),
	}
	// Should not panic
	s.remindIntervieweesDayBefore()
}

func TestRemindDayBefore_NoInterviews(t *testing.T) {
	repo := new(mocks.MockIrisJobsRepository)
	s := testScheduler(repo)

	repo.On("GetTomorrowInterviews", mock.Anything).Return([]iris.InterviewReminder{}, nil)

	s.remindIntervieweesDayBefore()

	repo.AssertExpectations(t)
}

// ── BuildJoinLink ──────────────────────────────────────────

func TestBuildJoinLink(t *testing.T) {
	link := buildJoinLink("https://conf.example.com/", "abc123", "part456")
	assert.Contains(t, link, "abc123")
	assert.Contains(t, link, "commsHash=part456")
}

func TestBuildJoinLink_EmptyBase(t *testing.T) {
	link := buildJoinLink("", "abc", "def")
	assert.Equal(t, "", link)
}
