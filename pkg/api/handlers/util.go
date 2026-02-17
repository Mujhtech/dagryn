package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	ProjectIDParam      = "projectId"
	InstallationIDParam = "installationId"
	UserIDParam         = "userId"
	RunIDParam          = "runId"
	CacheKeyParam       = "cacheKey"
	InvitationIDParam   = "invitationId"
	ArtifactIDParam     = "artifactId"
	TaskNameParam       = "taskName"
	PluginNameParam     = "pluginName"
	KeyIDParam          = "keyId"
	PublisherParam      = "publisher"
	NameParam           = "name"
	VersionParam        = "version"
	SlugParam           = "slug"
)

func pathParamOrError(r *http.Request, paramName string) (string, error) {
	val, ok := pathParam(r, paramName)
	if !ok {
		return "", fmt.Errorf("parameter '%s' not found in request path", paramName)
	}

	return val, nil
}

func pathParam(r *http.Request, paramName string) (string, bool) {
	val := chi.URLParam(r, paramName)
	if val == "" {
		return "", false
	}

	return val, true
}

func getProjectIDFromPath(r *http.Request) (uuid.UUID, error) {
	projectId, err := pathParamOrError(r, ProjectIDParam)
	if err != nil {
		return uuid.Nil, fmt.Errorf("project ID is required")
	}

	projectID, err := uuid.Parse(projectId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid project ID")
	}

	return projectID, nil
}

func getRunIDFromPath(r *http.Request) (uuid.UUID, error) {
	runId, err := pathParamOrError(r, RunIDParam)
	if err != nil {
		return uuid.Nil, fmt.Errorf("run ID is required")
	}

	runID, err := uuid.Parse(runId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid run ID")
	}

	return runID, nil
}

func getInstallationIDFromPath(r *http.Request) (uuid.UUID, error) {
	installationId, err := pathParamOrError(r, InstallationIDParam)
	if err != nil {
		return uuid.Nil, fmt.Errorf("installation ID is required")
	}

	installationID, err := uuid.Parse(installationId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid installation ID")
	}

	return installationID, nil
}

func getUserIDFromPath(r *http.Request) (uuid.UUID, error) {
	userId, err := pathParamOrError(r, UserIDParam)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user ID is required")
	}

	userID, err := uuid.Parse(userId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID")
	}

	return userID, nil
}

func getTaskNameFromPath(r *http.Request) (string, error) {
	taskName, err := pathParamOrError(r, TaskNameParam)
	if err != nil {
		return "", fmt.Errorf("task name is required")
	}

	return taskName, nil
}
