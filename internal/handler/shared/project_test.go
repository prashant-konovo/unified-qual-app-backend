package shared_test

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/shared"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/unittests"
	"github.com/InCrowd/unified-qual-api/internal/unittests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// listResponse mirrors utilities.PaginatedResponse for test decoding.
type listResponse struct {
	Success bool             `json:"success"`
	Data    []map[string]any `json:"data"`
	Meta    struct {
		Page       int `json:"page"`
		PageSize   int `json:"pageSize"`
		TotalCount int `json:"totalCount"`
	} `json:"meta"`
}

var fixedTime = time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

func sampleIRISRow(id int64, name string) iris.ProjectListRow {
	return iris.ProjectListRow{
		ID:              id,
		Name:            name,
		SubscriptionID:  100,
		ProjectStatusID: 1,
		ProjectStatusName: "Active",
		ProjectTypeID:   1,
		CreatedOn:       fixedTime,
	}
}

func sampleQSRow(id int64, name string) qs.ProjectListRow {
	return qs.ProjectListRow{
		ID:                id,
		Name:              name,
		SalesforceJobNumber: sql.NullString{String: "SF-001", Valid: true},
		ProjectStatusID:  1,
		ProjectStatusName: "Active",
		CreatedOn:        fixedTime,
	}
}

func TestListProjects_BothSources(t *testing.T) {
	deps := unittests.TestDeps()

	irisRepo := new(mocks.MockIrisProjectRepository)
	qsRepo := new(mocks.MockQsProjectRepository)

	irisRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{sampleIRISRow(1, "IRIS Project")}, 1, nil)
	qsRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]qs.ProjectListRow{sampleQSRow(2, "QS Project")}, 1, nil)

	deps.ProjectService = service.NewProjectService(irisRepo, qsRepo, nil)

	handler := &shared.ProjectHandler{Deps: deps}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)

	handler.ListProjects(rec, req)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var resp listResponse
	unittests.DecodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 1, resp.Meta.Page)
	assert.Equal(t, 20, resp.Meta.PageSize)
	assert.Equal(t, 2, resp.Meta.TotalCount)

	// First item from IRIS
	assert.Equal(t, float64(1), resp.Data[0]["id"])
	assert.Equal(t, "IRIS Project", resp.Data[0]["name"])
	assert.Equal(t, "iris", resp.Data[0]["source"])
	assert.Equal(t, "LS", resp.Data[0]["serviceCategory"])

	// Second item from QS
	assert.Equal(t, float64(2), resp.Data[1]["id"])
	assert.Equal(t, "QS Project", resp.Data[1]["name"])
	assert.Equal(t, "qs", resp.Data[1]["source"])
	assert.Equal(t, "MRA", resp.Data[1]["serviceCategory"])

	irisRepo.AssertExpectations(t)
	qsRepo.AssertExpectations(t)
}

func TestListProjects_IRISOnly(t *testing.T) {
	deps := unittests.TestDeps()

	irisRepo := new(mocks.MockIrisProjectRepository)
	qsRepo := new(mocks.MockQsProjectRepository)

	irisRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{sampleIRISRow(10, "Only IRIS")}, 1, nil)

	deps.ProjectService = service.NewProjectService(irisRepo, qsRepo, nil)

	handler := &shared.ProjectHandler{Deps: deps}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects?source=iris", nil)

	handler.ListProjects(rec, req)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var resp listResponse
	unittests.DecodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "iris", resp.Data[0]["source"])
	assert.Equal(t, "Only IRIS", resp.Data[0]["name"])
	assert.Equal(t, 1, resp.Meta.TotalCount)

	irisRepo.AssertExpectations(t)
	qsRepo.AssertNotCalled(t, "List")
}

func TestListProjects_QSOnly(t *testing.T) {
	deps := unittests.TestDeps()

	irisRepo := new(mocks.MockIrisProjectRepository)
	qsRepo := new(mocks.MockQsProjectRepository)

	qsRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]qs.ProjectListRow{sampleQSRow(20, "Only QS")}, 1, nil)

	deps.ProjectService = service.NewProjectService(irisRepo, qsRepo, nil)

	handler := &shared.ProjectHandler{Deps: deps}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects?serviceCategory=MRA", nil)

	handler.ListProjects(rec, req)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var resp listResponse
	unittests.DecodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "qs", resp.Data[0]["source"])
	assert.Equal(t, "MRA", resp.Data[0]["serviceCategory"])
	assert.Equal(t, "Only QS", resp.Data[0]["name"])
	assert.Equal(t, 1, resp.Meta.TotalCount)

	irisRepo.AssertNotCalled(t, "List")
	qsRepo.AssertExpectations(t)
}

func TestListProjects_EmptyResult(t *testing.T) {
	deps := unittests.TestDeps()

	irisRepo := new(mocks.MockIrisProjectRepository)
	qsRepo := new(mocks.MockQsProjectRepository)

	irisRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{}, 0, nil)
	qsRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]qs.ProjectListRow{}, 0, nil)

	deps.ProjectService = service.NewProjectService(irisRepo, qsRepo, nil)

	handler := &shared.ProjectHandler{Deps: deps}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)

	handler.ListProjects(rec, req)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var resp listResponse
	unittests.DecodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data, "data should be an empty array, not null")
	assert.Len(t, resp.Data, 0)
	assert.Equal(t, 0, resp.Meta.TotalCount)

	irisRepo.AssertExpectations(t)
	qsRepo.AssertExpectations(t)
}

func TestListProjects_IRISError(t *testing.T) {
	deps := unittests.TestDeps()

	irisRepo := new(mocks.MockIrisProjectRepository)
	qsRepo := new(mocks.MockQsProjectRepository)

	irisRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow(nil), 0, errors.New("iris db down"))
	qsRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]qs.ProjectListRow{sampleQSRow(5, "Surviving QS")}, 1, nil)

	deps.ProjectService = service.NewProjectService(irisRepo, qsRepo, nil)

	handler := &shared.ProjectHandler{Deps: deps}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)

	handler.ListProjects(rec, req)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var resp listResponse
	unittests.DecodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1, "should contain only QS results when IRIS errors")
	assert.Equal(t, "qs", resp.Data[0]["source"])
	assert.Equal(t, "Surviving QS", resp.Data[0]["name"])
	assert.Equal(t, 1, resp.Meta.TotalCount)

	irisRepo.AssertExpectations(t)
	qsRepo.AssertExpectations(t)
}

func TestListProjects_NilRepos(t *testing.T) {
	deps := unittests.TestDeps()
	deps.ProjectService = service.NewProjectService(nil, nil, nil)

	handler := &shared.ProjectHandler{Deps: deps}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)

	handler.ListProjects(rec, req)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var resp listResponse
	unittests.DecodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data, "data should be an empty array, not null")
	assert.Len(t, resp.Data, 0)
	assert.Equal(t, 0, resp.Meta.TotalCount)
}
