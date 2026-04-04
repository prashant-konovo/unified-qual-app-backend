package jobs

import "os"

// JobsConfig holds per-job cron expressions and the master enable switch.
// All cron expressions use robfig/cron/v3 6-field format: second minute hour dom month dow
type JobsConfig struct {
	Enabled                 bool
	CronIssueQualHonorarium string
	CronCompleteProjects    string
	CronEndConferences      string
	CronCloseSurvey         string
	CronMonthlyTranscripts  string
	CronRemindDayBefore     string
	CronRemind30MinBefore   string
	CronRemindConfirm       string
	CronCalendarWatch       string
	QualConfBaseURL         string
	SystemUserID            int64
}

// DefaultJobsConfig returns a JobsConfig with production-matching defaults.
func DefaultJobsConfig() JobsConfig {
	return JobsConfig{
		Enabled:                 envBool("JOBS_ENABLED", false),
		CronIssueQualHonorarium: envOrDefault("JOB_CRON_ISSUE_QUAL_HONORARIUM", "0 0 */6 * * *"),
		CronCompleteProjects:    envOrDefault("JOB_CRON_COMPLETE_PROJECTS", "0 0 */6 * * *"),
		CronEndConferences:      envOrDefault("JOB_CRON_END_CONFERENCES", "0 */5 * * * *"),
		CronCloseSurvey:         envOrDefault("JOB_CRON_CLOSE_SURVEY", "0 0 */6 * * *"),
		CronMonthlyTranscripts:  envOrDefault("JOB_CRON_MONTHLY_TRANSCRIPTS", "0 0 3 1 * *"),
		CronRemindDayBefore:     envOrDefault("JOB_CRON_REMIND_DAY_BEFORE", "0 0 11 * * *"),
		CronRemind30MinBefore:   envOrDefault("JOB_CRON_REMIND_30MIN_BEFORE", "0 * * * * *"),
		CronRemindConfirm:       envOrDefault("JOB_CRON_REMIND_CONFIRM_SCHEDULE", "0 0 11 * * *"),
		CronCalendarWatch:       envOrDefault("JOB_CRON_UPDATE_CALENDAR_WATCH", "0 0 0 */20 * *"),
		QualConfBaseURL:         envOrDefault("QUAL_CONF_BASE_URL", ""),
		SystemUserID:            1,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "true" || v == "1" || v == "yes" {
		return true
	}
	if v == "false" || v == "0" || v == "no" {
		return false
	}
	return fallback
}
