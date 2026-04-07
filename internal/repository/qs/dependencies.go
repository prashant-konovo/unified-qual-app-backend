package qs

import "database/sql"

// Repositories holds all QS repository instances.
type Repositories struct {
	Project    ProjectRepository
	User       UserRepository
	TimeSlot   TimeSlotRepository
	Respondent RespondentRepository
	Survey     *SurveyRepo // concrete for EnsureTable
	Conference ConferenceRepository
	Answer     AnswerRepository
	Interviews InterviewsRepository
}

// NewRepositories creates all QS repository instances from a database connection.
func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Project:    NewProjectRepo(db),
		User:       NewUserRepo(db),
		TimeSlot:   NewTimeSlotRepo(db),
		Respondent: NewRespondentRepo(db),
		Survey:     NewSurveyRepo(db),
		Conference: NewConferenceRepo(db),
		Answer:     NewAnswerRepo(db),
		Interviews: NewInterviewsRepo(db),
	}
}
