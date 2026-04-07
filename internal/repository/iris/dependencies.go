package iris

import "database/sql"

// Repositories holds all IRIS repository instances.
type Repositories struct {
	Project ProjectRepository
	User    UserRepository
	Survey  SurveyRepository
	Jobs    JobsRepository
}

// NewRepositories creates all IRIS repository instances from read-write and read-only DB connections.
func NewRepositories(rw, ro *sql.DB) *Repositories {
	return &Repositories{
		Project: NewProjectRepo(rw, ro),
		User:    NewUserRepo(rw, ro),
		Survey:  NewSurveyRepo(rw, ro),
		Jobs:    NewJobsRepo(rw, ro),
	}
}
