package jobs

import (
	"context"
)

// monthlyTranscriptsJob fetches CastingWords transcription totals for the
// previous month's completed transcripts and updates the billing records.
// Matches Scala: MonthlyTranscriptsJob.scala
// Schedule: 1st of month at 3am UTC
func (s *Scheduler) monthlyTranscriptsJob() {
	ctx := context.Background()
	log := s.logger.With("job", "MonthlyTranscriptsJob")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	transcripts, err := s.deps.JobsRepo.GetPreviousMonthTranscripts(ctx)
	if err != nil {
		log.Error("failed to get previous month transcripts", "error", err)
		return
	}
	log.Info("found transcripts to update", "count", len(transcripts))

	cw := s.deps.Services.CastingWords
	if cw == nil || !cw.Configured() {
		log.Warn("CastingWords client not configured, skipping")
		return
	}

	var nUpdated int
	for _, t := range transcripts {
		total, err := cw.GetAudiofileTotal(ctx, t.TranscriptAudiofileID)
		if err != nil {
			log.Error("failed to get audiofile total",
				"id", t.ID, "audiofileID", t.TranscriptAudiofileID, "error", err)
			continue
		}
		if total == nil {
			log.Warn("no total returned", "audiofileID", t.TranscriptAudiofileID)
			continue
		}

		if err := s.deps.JobsRepo.UpdateTranscriptTotal(ctx, t.ID, *total); err != nil {
			log.Error("failed to update transcript total",
				"id", t.ID, "total", *total, "error", err)
			continue
		}
		log.Debug("updated transcript total",
			"audiofileID", t.TranscriptAudiofileID, "total", *total)
		nUpdated++
	}
	log.Info("completed", "updated", nUpdated, "total", len(transcripts))
}
