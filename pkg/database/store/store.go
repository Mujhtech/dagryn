package store

import (
	"github.com/mujhtech/dagryn/pkg/cache"
	"github.com/mujhtech/dagryn/pkg/database"
	"github.com/mujhtech/dagryn/pkg/database/repo"
)

type Store struct {
	Users               *repo.UserRepo
	Tokens              *repo.TokenRepo
	Teams               *repo.TeamRepo
	Projects            *repo.ProjectRepo
	APIKeys             *repo.APIKeyRepo
	Invitations         *repo.InvitationRepo
	Runs                *repo.RunRepo
	Artifacts           *repo.ArtifactRepo
	ProviderTokens      *repo.ProviderTokenRepo
	GitHubInstallations *repo.GitHubInstallationRepo
	Workflows           *repo.WorkflowRepo
	PluginRegistry      *repo.PluginRegistryRepo
	Billing             *repo.BillingRepo
	AI                  *repo.AIRepo
	Cache               *repo.CacheRepo
}

func New(
	cache cache.Cache,
	db *database.DB,
) Store {
	return Store{
		Users:               repo.NewUserRepo(db.Pool()),
		Tokens:              repo.NewTokenRepo(db.Pool()),
		Teams:               repo.NewTeamRepo(db.Pool()),
		Projects:            repo.NewProjectRepo(db.Pool()),
		APIKeys:             repo.NewAPIKeyRepo(db.Pool()),
		Invitations:         repo.NewInvitationRepo(db.Pool()),
		Runs:                repo.NewRunRepo(db.Pool()),
		Artifacts:           repo.NewArtifactRepo(db.Pool()),
		ProviderTokens:      repo.NewProviderTokenRepo(db.Pool()),
		GitHubInstallations: repo.NewGitHubInstallationRepo(db.Pool()),
		Workflows:           repo.NewWorkflowRepo(db.Pool()),
		PluginRegistry:      repo.NewPluginRegistryRepo(db.Pool()),
		Billing:             repo.NewBillingRepo(db.Pool()),
		AI:                  repo.NewAIRepo(db.Pool()),
		Cache:               repo.NewCacheRepo(db.Pool()),
	}
}
