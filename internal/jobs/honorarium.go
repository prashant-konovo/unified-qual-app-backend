package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// issueQualHonorarium pays participants for completed qual interviews.
// Matches Scala: PaymentManager.IssueQualHonorarium
// Schedule: every 6 hours
func (s *Scheduler) issueQualHonorarium() {
	ctx := context.Background()
	log := s.logger.With("job", "IssueQualHonorarium")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	slots, err := s.deps.JobsRepo.GetPayableTimeSlots(ctx)
	if err != nil {
		log.Error("failed to get payable time slots", "error", err)
		return
	}
	log.Info("found payable time slots", "count", len(slots))

	var nPaid int
	for _, ts := range slots {
		if err := s.processHonorarium(ctx, log, ts); err != nil {
			log.Error("failed to process honorarium",
				"timeSlotID", ts.TimeSlotID, "userID", ts.UserID, "error", err)
			continue
		}
		nPaid++
	}
	log.Info("completed", "paid", nPaid, "total", len(slots))
}

func (s *Scheduler) processHonorarium(ctx context.Context, log *slog.Logger, ts iris.PayableTimeSlot) error {
	// Eligibility: userType=1 (Responder), market pays rewards, honorarium > 0
	if ts.UserTypeID != 1 || !ts.MarketRewards || ts.PromisedHonorarium <= 0 {
		log.Info("ineligible for payment, marking completed",
			"timeSlotID", ts.TimeSlotID, "userTypeID", ts.UserTypeID,
			"marketRewards", ts.MarketRewards, "honorarium", ts.PromisedHonorarium)
		return s.deps.JobsRepo.MarkHonorariumPaid(ctx, ts.TimeSlotID, 0)
	}

	sfProjectID := ""
	if ts.SFProjectID.Valid {
		sfProjectID = ts.SFProjectID.String
	}

	reason := fmt.Sprintf("Given for interview at %s surveyId: (#%d), salesForceProjectId: (#%s)",
		ts.StartTime.Format("2006-01-02 15:04"), ts.SurveyID, sfProjectID)

	creditID, err := s.deps.JobsRepo.GrantCredit(ctx, iris.CreditParams{
		UserID:       ts.UserID,
		Amount:       ts.PromisedHonorarium,
		ReasonID:     7, // qual interview
		Reason:       reason,
		UserSurveyID: ts.UserSurveyID,
		SystemUserID: s.cfg.SystemUserID,
	})
	if err != nil {
		return fmt.Errorf("grant credit: %w", err)
	}

	if err := s.deps.JobsRepo.MarkHonorariumPaid(ctx, ts.TimeSlotID, creditID); err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}

	// Send email notification (best-effort)
	s.sendHonorariumEmail(ctx, log, ts)

	return nil
}

func (s *Scheduler) sendHonorariumEmail(ctx context.Context, log *slog.Logger, ts iris.PayableTimeSlot) {
	nc := s.deps.Services.Notification
	if nc == nil || !nc.Configured() {
		return
	}

	subject := "Your interview payment has been issued"
	body := fmt.Sprintf(
		"Your payment of %d credits for your interview on %s has been issued. "+
			"Please check your dashboard for details.",
		ts.PromisedHonorarium, ts.StartTime.Format("Monday, January 2"))

	if err := nc.SendEmail(ctx, integration.EmailMessage{
		To:          []string{ts.UserEmail},
		Subject:     subject,
		Body:        body,
		ContentType: "text/html",
	}); err != nil {
		log.Warn("failed to send honorarium email",
			"timeSlotID", ts.TimeSlotID, "email", ts.UserEmail, "error", err)
	}
}
