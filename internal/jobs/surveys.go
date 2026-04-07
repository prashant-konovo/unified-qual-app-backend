package jobs

import (
	"context"
	"log/slog"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// closeSurveyJob auto-closes surveys whose Salesforce project status indicates
// fieldwork is complete. Matches Scala: CloseSurveyJob.scala
// Schedule: every 6 hours
func (s *Scheduler) closeSurveyJob() {
	ctx := context.Background()
	log := s.logger.With("job", "CloseSurveyJob")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	// Check feature flag
	enabled, err := s.deps.JobsRepo.GetFeatureFlag(ctx, "enableCloseSurveyJob")
	if err != nil {
		log.Error("failed to check feature flag", "error", err)
		return
	}
	if enabled != 1 {
		log.Info("disabled by feature flag")
		return
	}

	surveys, err := s.deps.JobsRepo.GetSurveysToClose(ctx)
	if err != nil {
		log.Error("failed to get surveys to close", "error", err)
		return
	}
	if len(surveys) > 0 {
		log.Info("found surveys to close", "count", len(surveys))
	}

	var nProcessed int
	for _, survey := range surveys {
		if err := s.processSurveyClose(ctx, log, survey); err != nil {
			log.Error("failed to process survey",
				"surveyID", survey.SurveyID, "error", err)
			continue
		}
		nProcessed++
	}

	if nProcessed > 0 {
		log.Info("completed", "processed", nProcessed, "total", len(surveys))
	}
}

func (s *Scheduler) processSurveyClose(ctx context.Context, log *slog.Logger, survey iris.SurveyToClose) error {
	// Determine action: set to in-review or close
	if survey.NeedsToBeReviewed && !survey.IsInReview {
		if err := s.deps.JobsRepo.SetSurveyToInReview(ctx, survey.SurveyID); err != nil {
			return err
		}
		log.Info("set survey to in-review", "surveyID", survey.SurveyID)
	} else {
		if err := s.deps.JobsRepo.CloseSurveyByJob(ctx, survey.SurveyID); err != nil {
			return err
		}
		log.Info("closed survey", "surveyID", survey.SurveyID)
	}

	// Stop pre-validated list sampling if any crowd is type 8
	crowdTypes, err := s.deps.JobsRepo.GetSurveyCrowdTypes(ctx, survey.SurveyID)
	if err != nil {
		log.Warn("failed to get crowd types", "surveyID", survey.SurveyID, "error", err)
	} else {
		for _, t := range crowdTypes {
			if t == 8 {
				log.Info("would stop pre-validated list sampling", "surveyID", survey.SurveyID)
				// TODO: Call PreValidatedListService.stop when available
				break
			}
		}
	}

	// Log activity
	if err := s.deps.JobsRepo.LogActivity(ctx, iris.ActivityLogParams{
		ObjectType:   "survey",
		ObjectID:     survey.SurveyID,
		Action:       "Close Survey Job",
		SubID:        survey.SubscriptionID,
		Description:  survey.SFProjectStatus,
		IndirectDesc: survey.SFProjectID,
	}); err != nil {
		log.Warn("failed to log activity", "surveyID", survey.SurveyID, "error", err)
	}

	return nil
}
