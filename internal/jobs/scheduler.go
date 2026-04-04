package jobs

import (
	"log/slog"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/robfig/cron/v3"
)

// JobDeps holds all dependencies needed by scheduled jobs.
type JobDeps struct {
	Cfg             *config.Config
	DB              *config.DBPair
	Services        *integration.ServiceClients
	IrisSurveyRepo  iris.SurveyRepository
	IrisProjectRepo iris.ProjectRepository
	IrisUserRepo    iris.UserRepository
	JobsRepo        iris.JobsRepository
}

// Scheduler manages all scheduled background jobs.
type Scheduler struct {
	cron   *cron.Cron
	deps   *JobDeps
	cfg    config.JobsConfig
	logger *slog.Logger
}

// NewScheduler creates a new job scheduler.
func NewScheduler(deps *JobDeps, jobsCfg config.JobsConfig, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithSeconds(), cron.WithLogger(
			cron.PrintfLogger(slog.NewLogLogger(logger.Handler(), slog.LevelDebug)),
		)),
		deps:   deps,
		cfg:    jobsCfg,
		logger: logger,
	}
}

// Start registers all jobs and begins the cron scheduler.
func (s *Scheduler) Start() error {
	jobs := []struct {
		name string
		expr string
		fn   func()
	}{
		{"IssueQualHonorarium", s.cfg.CronIssueQualHonorarium, s.issueQualHonorarium},
		{"CompleteProjects", s.cfg.CronCompleteProjects, s.completeProjects},
		{"EndConferences", s.cfg.CronEndConferences, s.endConferences},
		{"CloseSurveyJob", s.cfg.CronCloseSurvey, s.closeSurveyJob},
		{"MonthlyTranscriptsJob", s.cfg.CronMonthlyTranscripts, s.monthlyTranscriptsJob},
		{"RemindIntervieweesDayBefore", s.cfg.CronRemindDayBefore, s.remindIntervieweesDayBefore},
		{"RemindInterviewees30MinBefore", s.cfg.CronRemind30MinBefore, s.remindInterviewees30MinBefore},
		{"RemindToConfirmSchedule", s.cfg.CronRemindConfirm, s.remindToConfirmSchedule},
		{"UpdateCalendarPushWatch", s.cfg.CronCalendarWatch, s.updateCalendarPushWatch},
	}

	for _, j := range jobs {
		if j.expr == "" {
			s.logger.Info("job disabled (empty cron expression)", "job", j.name)
			continue
		}
		if _, err := s.cron.AddFunc(j.expr, j.fn); err != nil {
			return err
		}
		s.logger.Info("registered job", "job", j.name, "schedule", j.expr)
	}

	s.cron.Start()
	s.logger.Info("job scheduler started", "jobCount", len(s.cron.Entries()))
	return nil
}

// Stop gracefully shuts down the scheduler, waiting for running jobs to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("job scheduler stopped")
}
