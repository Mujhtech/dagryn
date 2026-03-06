package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/rs/zerolog"
)

// PluginSearchResult holds paginated plugin search results.
type PluginSearchResult struct {
	Plugins []*models.RegistryPluginWithPublisher `json:"plugins"`
	Total   int                                   `json:"total"`
	Page    int                                   `json:"page"`
	PerPage int                                   `json:"per_page"`
}

// PluginDetailResult holds detailed plugin information.
type PluginDetailResult struct {
	Plugin   *models.RegistryPluginWithPublisher `json:"plugin"`
	Versions []*models.PluginVersion             `json:"versions"`
}

// PluginAnalytics holds download analytics for a plugin.
type PluginAnalytics struct {
	TotalDownloads  int64               `json:"total_downloads"`
	WeeklyDownloads int64               `json:"weekly_downloads"`
	DailyStats      []repo.DownloadStat `json:"daily_stats"`
}

// PluginRegistryService coordinates plugin registry operations.
type PluginRegistryService struct {
	repo   repo.PluginRegistryStore
	logger zerolog.Logger
}

// NewPluginRegistryService creates a new plugin registry service.
func NewPluginRegistryService(registryRepo repo.PluginRegistryStore, logger zerolog.Logger) *PluginRegistryService {
	return &PluginRegistryService{
		repo:   registryRepo,
		logger: logger.With().Str("service", "plugin_registry").Logger(),
	}
}

// SearchPlugins searches the registry with pagination.
func (s *PluginRegistryService) SearchPlugins(ctx context.Context, query, pluginType, publisher, sort string, page, perPage int) (*PluginSearchResult, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	result, err := s.repo.SearchPlugins(ctx, repo.PluginSearchParams{
		Query:     query,
		Type:      pluginType,
		Publisher: publisher,
		Sort:      sort,
		Limit:     perPage,
		Offset:    (page - 1) * perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("search plugins: %w", err)
	}

	plugins := result.Plugins
	if plugins == nil {
		plugins = []*models.RegistryPluginWithPublisher{}
	}

	return &PluginSearchResult{
		Plugins: plugins,
		Total:   result.Total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// GetPluginDetail returns full plugin info with versions.
func (s *PluginRegistryService) GetPluginDetail(ctx context.Context, publisher, name string) (*PluginDetailResult, error) {
	plugin, err := s.repo.GetPluginByPublisherAndName(ctx, publisher, name)
	if err != nil {
		return nil, fmt.Errorf("get plugin: %w", err)
	}

	versions, err := s.repo.ListVersions(ctx, plugin.ID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	return &PluginDetailResult{
		Plugin:   plugin,
		Versions: versions,
	}, nil
}

// PublishVersion creates a new version for a plugin.
func (s *PluginRegistryService) PublishVersion(ctx context.Context, publisher, name string, manifestJSON json.RawMessage, version, releaseNotes string) error {
	pub, err := s.repo.GetPublisherByName(ctx, publisher)
	if err != nil {
		return fmt.Errorf("publisher not found: %w", err)
	}

	plugin, err := s.repo.GetPluginByPublisherAndName(ctx, publisher, name)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	// Verify ownership
	_ = pub // ownership checks can be added later

	v := &models.PluginVersion{
		PluginID:     plugin.ID,
		Version:      version,
		ManifestJSON: manifestJSON,
		ReleaseNotes: &releaseNotes,
	}

	if _, err := s.repo.CreateVersion(ctx, v); err != nil {
		return fmt.Errorf("create version: %w", err)
	}

	// Update latest version on plugin
	plugin.LatestVersion = version
	if err := s.repo.UpdatePlugin(ctx, &plugin.RegistryPlugin); err != nil {
		s.logger.Warn().Err(err).Msg("failed to update latest version")
	}

	s.logger.Info().
		Str("publisher", publisher).
		Str("plugin", name).
		Str("version", version).
		Msg("plugin version published")

	return nil
}

// RecordDownload records a download and increments counters.
func (s *PluginRegistryService) RecordDownload(ctx context.Context, publisher, name, version string, userID *uuid.UUID, ipHash string) error {
	plugin, err := s.repo.GetPluginByPublisherAndName(ctx, publisher, name)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	v, err := s.repo.GetVersion(ctx, plugin.ID, version)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	d := &models.PluginDownload{
		PluginID:  plugin.ID,
		VersionID: v.ID,
		UserID:    userID,
	}
	if ipHash != "" {
		d.IPHash = &ipHash
	}

	if err := s.repo.RecordDownload(ctx, d); err != nil {
		return fmt.Errorf("record download: %w", err)
	}

	if err := s.repo.IncrementDownloads(ctx, plugin.ID, v.ID); err != nil {
		s.logger.Warn().Err(err).Msg("failed to increment download counters")
	}

	return nil
}

// GetAnalytics returns download analytics for a plugin.
func (s *PluginRegistryService) GetAnalytics(ctx context.Context, publisher, name string, days int) (*PluginAnalytics, error) {
	plugin, err := s.repo.GetPluginByPublisherAndName(ctx, publisher, name)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}

	stats, err := s.repo.GetDownloadStats(ctx, plugin.ID, days)
	if err != nil {
		return nil, fmt.Errorf("get download stats: %w", err)
	}

	return &PluginAnalytics{
		TotalDownloads:  plugin.TotalDownloads,
		WeeklyDownloads: plugin.WeeklyDownloads,
		DailyStats:      stats,
	}, nil
}

// ListFeatured returns featured plugins.
func (s *PluginRegistryService) ListFeatured(ctx context.Context, limit int) ([]*models.RegistryPluginWithPublisher, error) {
	return s.repo.ListFeatured(ctx, limit)
}

// ListTrending returns trending plugins.
func (s *PluginRegistryService) ListTrending(ctx context.Context, limit int) ([]*models.RegistryPluginWithPublisher, error) {
	return s.repo.ListTrending(ctx, limit, 7)
}

// GetPublisher returns a publisher by name.
func (s *PluginRegistryService) GetPublisher(ctx context.Context, name string) (*models.PluginPublisher, error) {
	return s.repo.GetPublisherByName(ctx, name)
}

// CreatePublisher creates a new publisher.
func (s *PluginRegistryService) CreatePublisher(ctx context.Context, p *models.PluginPublisher) (*models.PluginPublisher, error) {
	return s.repo.CreatePublisher(ctx, p)
}

// ListVersions returns all versions for a plugin.
func (s *PluginRegistryService) ListVersions(ctx context.Context, publisher, name string) ([]*models.PluginVersion, error) {
	plugin, err := s.repo.GetPluginByPublisherAndName(ctx, publisher, name)
	if err != nil {
		return nil, err
	}
	return s.repo.ListVersions(ctx, plugin.ID)
}

// YankVersion marks a version as yanked.
func (s *PluginRegistryService) YankVersion(ctx context.Context, publisher, name, version string) error {
	plugin, err := s.repo.GetPluginByPublisherAndName(ctx, publisher, name)
	if err != nil {
		return err
	}
	return s.repo.YankVersion(ctx, plugin.ID, version)
}

// RecomputeWeeklyDownloads updates weekly download counts for all plugins.
func (s *PluginRegistryService) RecomputeWeeklyDownloads(ctx context.Context) error {
	return s.repo.RecomputeWeeklyDownloads(ctx)
}
