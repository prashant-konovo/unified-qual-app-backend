package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	mocks "github.com/InCrowd/unified-qual-api/internal/unittests/mocks"
)

// ── List tests ──────────────────────────────────────────────────────────────

func TestListProjects_BothSources(t *testing.T) {
	irisRepo := new(mocks.MockIrisProjectRepository)
	qsRepo := new(mocks.MockQsProjectRepository)

	irisRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{{ID: 1, Name: "IRIS"}}, 1, nil)
	qsRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]qs.ProjectListRow{{ID: 2, Name: "QS"}}, 1, nil)

	svc := NewProjectService(irisRepo, qsRepo, nil)
	result := svc.ListProjects(context.Background(), 1, 20, "", "", nil)

	assert.Len(t, result.Projects, 2)
	assert.Equal(t, 2, result.Total)
}

func TestListProjects_IRISOnly(t *testing.T) {
	irisRepo := new(mocks.MockIrisProjectRepository)

	irisRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{{ID: 1, Name: "IRIS"}}, 1, nil)

	svc := NewProjectService(irisRepo, nil, nil)
	result := svc.ListProjects(context.Background(), 1, 20, "iris", "", nil)

	assert.Len(t, result.Projects, 1)
}

func TestListProjects_NilRepos(t *testing.T) {
	svc := NewProjectService(nil, nil, nil)
	result := svc.ListProjects(context.Background(), 1, 20, "", "", nil)

	assert.NotNil(t, result.Projects)
	assert.Len(t, result.Projects, 0)
}

// ── Get tests ───────────────────────────────────────────────────────────────

func TestGetProject_QSFound(t *testing.T) {
	qsRepo := new(mocks.MockQsProjectRepository)
	qsRepo.On("GetByID", mock.Anything, int64(1)).Return(&qs.Project{
		ID: 1, Name: "Test Project", ProjectStatusID: 3,
	}, nil)
	qsRepo.On("TimeSlotCounts", mock.Anything, int64(1)).Return(5, 3, nil)
	qsRepo.On("GetTopics", mock.Anything, int64(1)).Return([]qs.Topic{{TopicName: "Health"}}, nil)

	svc := NewProjectService(nil, qsRepo, nil)
	result, err := svc.GetProject(context.Background(), 1, "")

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "Test Project", result.Project["name"])
}

func TestGetProject_IRISFound(t *testing.T) {
	irisRepo := new(mocks.MockIrisProjectRepository)
	irisRepo.On("GetByID", mock.Anything, int64(1)).Return(&iris.Project{
		ID: 1, Name: "IRIS Project", ProjectStatusID: 3,
	}, nil)

	svc := NewProjectService(irisRepo, nil, nil)
	result, err := svc.GetProject(context.Background(), 1, "iris")

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "IRIS Project", result.Project["name"])
	assert.Equal(t, "In Progress", result.Project["status"])
}

func TestGetProject_NotFound(t *testing.T) {
	qsRepo := new(mocks.MockQsProjectRepository)
	qsRepo.On("GetByID", mock.Anything, int64(999)).Return((*qs.Project)(nil), nil)

	irisRepo := new(mocks.MockIrisProjectRepository)
	irisRepo.On("GetByID", mock.Anything, int64(999)).Return((*iris.Project)(nil), nil)

	svc := NewProjectService(irisRepo, qsRepo, nil)
	result, err := svc.GetProject(context.Background(), 999, "")

	assert.NoError(t, err)
	assert.False(t, result.Found)
}

// ── Create tests ────────────────────────────────────────────────────────────

func TestCreateProject_QSDefault(t *testing.T) {
	qsRepo := new(mocks.MockQsProjectRepository)
	qsRepo.On("Create", mock.Anything, mock.AnythingOfType("*qs.Project")).Return(int64(42), nil)

	svc := NewProjectService(nil, qsRepo, nil)
	result, err := svc.CreateProject(context.Background(), dto.CreateProjectRequest{
		Name:   "New Project",
		Source: "",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(42), result.ID)
	assert.Equal(t, "qs", result.Source)
	assert.Equal(t, "MRA", result.ServiceCategory)
}

func TestCreateProject_IRIS(t *testing.T) {
	irisRepo := new(mocks.MockIrisProjectRepository)
	irisRepo.On("Create", mock.Anything, mock.AnythingOfType("*iris.Project")).Return(int64(10), nil)

	svc := NewProjectService(irisRepo, nil, nil)
	result, err := svc.CreateProject(context.Background(), dto.CreateProjectRequest{
		Name:   "IRIS Project",
		Source: "iris",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(10), result.ID)
	assert.Equal(t, "iris", result.Source)
	assert.Equal(t, "LS", result.ServiceCategory)
}

func TestCreateProject_NoDB(t *testing.T) {
	svc := NewProjectService(nil, nil, nil)
	_, err := svc.CreateProject(context.Background(), dto.CreateProjectRequest{Name: "X"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no database available")
}

// ── Update tests ────────────────────────────────────────────────────────────

func TestUpdateProject_QS(t *testing.T) {
	qsRepo := new(mocks.MockQsProjectRepository)
	qsRepo.On("Update", mock.Anything, int64(1), mock.Anything).Return(nil)

	svc := NewProjectService(nil, qsRepo, nil)
	size := int64(50)
	result, err := svc.UpdateProject(context.Background(), 1, dto.UpdateProjectRequest{
		Name:       "Updated",
		SampleSize: &size,
	})

	assert.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "qs", result.Source)
}

func TestUpdateProject_IRIS(t *testing.T) {
	irisRepo := new(mocks.MockIrisProjectRepository)
	irisRepo.On("Update", mock.Anything, int64(1), mock.Anything).Return(nil)

	svc := NewProjectService(irisRepo, nil, nil)
	result, err := svc.UpdateProject(context.Background(), 1, dto.UpdateProjectRequest{
		Name:   "Updated IRIS",
		Source: "iris",
	})

	assert.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "iris", result.Source)
}

func TestUpdateProject_Error(t *testing.T) {
	qsRepo := new(mocks.MockQsProjectRepository)
	qsRepo.On("Update", mock.Anything, int64(1), mock.Anything).Return(fmt.Errorf("db error"))

	svc := NewProjectService(nil, qsRepo, nil)
	_, err := svc.UpdateProject(context.Background(), 1, dto.UpdateProjectRequest{Name: "X"})
	assert.Error(t, err)
}

// ── Delete tests ────────────────────────────────────────────────────────────

func TestDeleteProject_QS(t *testing.T) {
	qsRepo := new(mocks.MockQsProjectRepository)
	qsRepo.On("Update", mock.Anything, int64(1), map[string]any{"project_status_id": 5}).Return(nil)

	svc := NewProjectService(nil, qsRepo, nil)
	result, err := svc.DeleteProject(context.Background(), 1, "qs")

	assert.NoError(t, err)
	assert.True(t, result.Archived)
	assert.Equal(t, "qs", result.Source)
}

func TestDeleteProject_IRIS(t *testing.T) {
	irisRepo := new(mocks.MockIrisProjectRepository)
	irisRepo.On("Update", mock.Anything, int64(1), map[string]any{"is_archived": true}).Return(nil)

	svc := NewProjectService(irisRepo, nil, nil)
	result, err := svc.DeleteProject(context.Background(), 1, "iris")

	assert.NoError(t, err)
	assert.True(t, result.Archived)
}

// ── ResolveSource tests ─────────────────────────────────────────────────────

func TestResolveSource(t *testing.T) {
	assert.Equal(t, "iris", ResolveSource("iris", ""))
	assert.Equal(t, "qs", ResolveSource("qs", ""))
	assert.Equal(t, "iris", ResolveSource("", "LS"))
	assert.Equal(t, "qs", ResolveSource("", "MRA"))
	assert.Equal(t, "", ResolveSource("", ""))
	assert.Equal(t, "custom", ResolveSource("custom", "LS"))
}

// ── Iris status name test ───────────────────────────────────────────────────

func TestIrisStatusName(t *testing.T) {
	assert.Equal(t, "Inquiry", irisStatusName[1])
	assert.Equal(t, "Defining", irisStatusName[2])
	assert.Equal(t, "In Progress", irisStatusName[3])
	assert.Equal(t, "Complete", irisStatusName[4])
	assert.Equal(t, "", irisStatusName[5]) // unknown
}
