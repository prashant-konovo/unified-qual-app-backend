package jobs

import (
	"context"
)

// completeProjects auto-completes finalized projects after 72 hours.
// Matches Scala: QualInterviewManager.CompleteProjects
// Schedule: every 6 hours
func (s *Scheduler) completeProjects() {
	ctx := context.Background()
	log := s.logger.With("job", "CompleteProjects")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	projects, err := s.deps.JobsRepo.GetProjectsToComplete(ctx)
	if err != nil {
		log.Error("failed to get projects to complete", "error", err)
		return
	}
	if len(projects) == 0 {
		log.Info("no projects to complete")
		return
	}

	log.Info("found projects to complete", "count", len(projects))

	var nCompleted int
	for _, p := range projects {
		if err := s.deps.JobsRepo.CompleteProject(ctx, p.ID); err != nil {
			log.Error("failed to complete project", "projectID", p.ID, "error", err)
			continue
		}
		nCompleted++
	}
	log.Info("completed", "completed", nCompleted, "total", len(projects))
}
