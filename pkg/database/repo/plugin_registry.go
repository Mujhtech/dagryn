package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mujhtech/dagryn/pkg/database/models"
)

// PluginRegistryRepo handles plugin registry database operations.
type PluginRegistryRepo struct {
	pool *pgxpool.Pool
}

// NewPluginRegistryRepo creates a new plugin registry repository.
func NewPluginRegistryRepo(pool *pgxpool.Pool) PluginRegistryStore {
	return &PluginRegistryRepo{pool: pool}
}

// GetPublisherByName returns a publisher by slug name.
func (r *PluginRegistryRepo) GetPublisherByName(ctx context.Context, name string) (*models.PluginPublisher, error) {
	var p models.PluginPublisher
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, display_name, avatar_url, website, verified, user_id, created_at, updated_at
		FROM plugin_publishers WHERE name = $1
	`, name).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.AvatarURL, &p.Website, &p.Verified, &p.UserID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// GetPublisherByID returns a publisher by ID.
func (r *PluginRegistryRepo) GetPublisherByID(ctx context.Context, id uuid.UUID) (*models.PluginPublisher, error) {
	var p models.PluginPublisher
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, display_name, avatar_url, website, verified, user_id, created_at, updated_at
		FROM plugin_publishers WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.AvatarURL, &p.Website, &p.Verified, &p.UserID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// CreatePublisher inserts a new publisher.
func (r *PluginRegistryRepo) CreatePublisher(ctx context.Context, p *models.PluginPublisher) (*models.PluginPublisher, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO plugin_publishers (id, name, display_name, avatar_url, website, verified, user_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, p.ID, p.Name, p.DisplayName, p.AvatarURL, p.Website, p.Verified, p.UserID, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ListPublishers returns all publishers.
func (r *PluginRegistryRepo) ListPublishers(ctx context.Context) ([]*models.PluginPublisher, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, display_name, avatar_url, website, verified, user_id, created_at, updated_at
		FROM plugin_publishers ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var publishers []*models.PluginPublisher
	for rows.Next() {
		var p models.PluginPublisher
		if err := rows.Scan(&p.ID, &p.Name, &p.DisplayName, &p.AvatarURL, &p.Website, &p.Verified, &p.UserID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		publishers = append(publishers, &p)
	}
	return publishers, rows.Err()
}

// GetPluginByPublisherAndName returns a plugin with publisher info.
func (r *PluginRegistryRepo) GetPluginByPublisherAndName(ctx context.Context, publisherName, pluginName string) (*models.RegistryPluginWithPublisher, error) {
	var p models.RegistryPluginWithPublisher
	err := r.pool.QueryRow(ctx, `
		SELECT rp.id, rp.publisher_id, rp.name, rp.description, rp.type, rp.license, rp.homepage,
		       rp.repository_url, rp.readme, rp.total_downloads, rp.weekly_downloads, rp.stars, rp.featured,
		       rp.deprecated, rp.latest_version, rp.created_at, rp.updated_at,
		       pp.name AS publisher_name, pp.verified AS publisher_verified
		FROM registry_plugins rp
		JOIN plugin_publishers pp ON pp.id = rp.publisher_id
		WHERE pp.name = $1 AND rp.name = $2
	`, publisherName, pluginName).Scan(
		&p.ID, &p.PublisherID, &p.Name, &p.Description, &p.Type, &p.License, &p.Homepage,
		&p.RepositoryURL, &p.Readme, &p.TotalDownloads, &p.WeeklyDownloads, &p.Stars, &p.Featured,
		&p.Deprecated, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
		&p.PublisherName, &p.PublisherVerified,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// GetPluginByID returns a plugin by ID.
func (r *PluginRegistryRepo) GetPluginByID(ctx context.Context, id uuid.UUID) (*models.RegistryPlugin, error) {
	var p models.RegistryPlugin
	err := r.pool.QueryRow(ctx, `
		SELECT id, publisher_id, name, description, type, license, homepage, repository_url,
		       readme, total_downloads, weekly_downloads, stars, featured, deprecated, latest_version,
		       created_at, updated_at
		FROM registry_plugins WHERE id = $1
	`, id).Scan(
		&p.ID, &p.PublisherID, &p.Name, &p.Description, &p.Type, &p.License, &p.Homepage,
		&p.RepositoryURL, &p.Readme, &p.TotalDownloads, &p.WeeklyDownloads, &p.Stars, &p.Featured,
		&p.Deprecated, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// CreatePlugin inserts a new registry plugin.
func (r *PluginRegistryRepo) CreatePlugin(ctx context.Context, p *models.RegistryPlugin) (*models.RegistryPlugin, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := r.pool.Exec(ctx, `
		INSERT INTO registry_plugins (id, publisher_id, name, description, type, license, homepage,
			repository_url, readme, total_downloads, weekly_downloads, stars, featured, deprecated,
			latest_version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, p.ID, p.PublisherID, p.Name, p.Description, p.Type, p.License, p.Homepage,
		p.RepositoryURL, p.Readme, p.TotalDownloads, p.WeeklyDownloads, p.Stars, p.Featured, p.Deprecated,
		p.LatestVersion, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePlugin updates a registry plugin.
func (r *PluginRegistryRepo) UpdatePlugin(ctx context.Context, p *models.RegistryPlugin) error {
	p.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE registry_plugins SET description=$1, type=$2, license=$3, homepage=$4,
			repository_url=$5, readme=$6, featured=$7, deprecated=$8, latest_version=$9, updated_at=$10
		WHERE id=$11
	`, p.Description, p.Type, p.License, p.Homepage, p.RepositoryURL,
		p.Readme, p.Featured, p.Deprecated, p.LatestVersion, p.UpdatedAt, p.ID)
	return err
}

// PluginSearchParams holds search parameters.
type PluginSearchParams struct {
	Query  string
	Type   string
	Sort   string // "name", "downloads", "updated"
	Limit  int
	Offset int
}

// PluginSearchResult holds paginated search results.
type PluginSearchResult struct {
	Plugins []*models.RegistryPluginWithPublisher
	Total   int
}

// SearchPlugins searches plugins with full-text and filtering.
func (r *PluginRegistryRepo) SearchPlugins(ctx context.Context, params PluginSearchParams) (*PluginSearchResult, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}

	// Build WHERE clause
	where := "WHERE 1=1"
	args := make([]interface{}, 0)
	argIdx := 1

	if params.Query != "" {
		where += ` AND to_tsvector('english', pp.name || ' ' || rp.name || ' ' || rp.description) @@ plainto_tsquery('english', $` + itoa(argIdx) + `)`
		args = append(args, params.Query)
		argIdx++
	}
	if params.Type != "" {
		where += ` AND rp.type = $` + itoa(argIdx)
		args = append(args, params.Type)
		argIdx++
	}

	// Count
	var total int
	countQuery := `SELECT COUNT(*) FROM registry_plugins rp JOIN plugin_publishers pp ON pp.id = rp.publisher_id ` + where
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Sort
	orderBy := "rp.name ASC"
	switch params.Sort {
	case "downloads":
		orderBy = "rp.total_downloads DESC"
	case "updated":
		orderBy = "rp.updated_at DESC"
	}

	// Fetch
	selectQuery := `
		SELECT rp.id, rp.publisher_id, rp.name, rp.description, rp.type, rp.license, rp.homepage,
		       rp.repository_url, rp.readme, rp.total_downloads, rp.weekly_downloads, rp.stars, rp.featured,
		       rp.deprecated, rp.latest_version, rp.created_at, rp.updated_at,
		       pp.name AS publisher_name, pp.verified AS publisher_verified
		FROM registry_plugins rp
		JOIN plugin_publishers pp ON pp.id = rp.publisher_id
		` + where + `
		ORDER BY ` + orderBy + `
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []*models.RegistryPluginWithPublisher
	for rows.Next() {
		var p models.RegistryPluginWithPublisher
		if err := rows.Scan(
			&p.ID, &p.PublisherID, &p.Name, &p.Description, &p.Type, &p.License, &p.Homepage,
			&p.RepositoryURL, &p.Readme, &p.TotalDownloads, &p.WeeklyDownloads, &p.Stars, &p.Featured,
			&p.Deprecated, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
			&p.PublisherName, &p.PublisherVerified,
		); err != nil {
			return nil, err
		}
		plugins = append(plugins, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &PluginSearchResult{Plugins: plugins, Total: total}, nil
}

// ListFeatured returns featured plugins.
func (r *PluginRegistryRepo) ListFeatured(ctx context.Context, limit int) ([]*models.RegistryPluginWithPublisher, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT rp.id, rp.publisher_id, rp.name, rp.description, rp.type, rp.license, rp.homepage,
		       rp.repository_url, rp.readme, rp.total_downloads, rp.weekly_downloads, rp.stars, rp.featured,
		       rp.deprecated, rp.latest_version, rp.created_at, rp.updated_at,
		       pp.name AS publisher_name, pp.verified AS publisher_verified
		FROM registry_plugins rp
		JOIN plugin_publishers pp ON pp.id = rp.publisher_id
		WHERE rp.featured = TRUE
		ORDER BY rp.total_downloads DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanPluginsWithPublisher(rows)
}

// ListTrending returns plugins with most downloads in recent days.
func (r *PluginRegistryRepo) ListTrending(ctx context.Context, limit, days int) ([]*models.RegistryPluginWithPublisher, error) {
	if limit <= 0 {
		limit = 10
	}
	if days <= 0 {
		days = 7
	}
	rows, err := r.pool.Query(ctx, `
		SELECT rp.id, rp.publisher_id, rp.name, rp.description, rp.type, rp.license, rp.homepage,
		       rp.repository_url, rp.readme, rp.total_downloads, rp.weekly_downloads, rp.stars, rp.featured,
		       rp.deprecated, rp.latest_version, rp.created_at, rp.updated_at,
		       pp.name AS publisher_name, pp.verified AS publisher_verified
		FROM registry_plugins rp
		JOIN plugin_publishers pp ON pp.id = rp.publisher_id
		LEFT JOIN (
			SELECT plugin_id, COUNT(*) AS recent_downloads
			FROM plugin_downloads
			WHERE created_at >= NOW() - INTERVAL '1 day' * $2
			GROUP BY plugin_id
		) pd ON pd.plugin_id = rp.id
		ORDER BY COALESCE(pd.recent_downloads, 0) DESC, rp.total_downloads DESC
		LIMIT $1
	`, limit, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanPluginsWithPublisher(rows)
}

// GetVersion returns a specific version of a plugin.
func (r *PluginRegistryRepo) GetVersion(ctx context.Context, pluginID uuid.UUID, version string) (*models.PluginVersion, error) {
	var v models.PluginVersion
	err := r.pool.QueryRow(ctx, `
		SELECT id, plugin_id, version, manifest_json, checksum_sha256, downloads, yanked, release_notes, published_at
		FROM plugin_versions WHERE plugin_id = $1 AND version = $2
	`, pluginID, version).Scan(
		&v.ID, &v.PluginID, &v.Version, &v.ManifestJSON, &v.ChecksumSHA256, &v.Downloads, &v.Yanked, &v.ReleaseNotes, &v.PublishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

// CreateVersion inserts a new plugin version.
func (r *PluginRegistryRepo) CreateVersion(ctx context.Context, v *models.PluginVersion) (*models.PluginVersion, error) {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.PublishedAt.IsZero() {
		v.PublishedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO plugin_versions (id, plugin_id, version, manifest_json, checksum_sha256, downloads, yanked, release_notes, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, v.ID, v.PluginID, v.Version, v.ManifestJSON, v.ChecksumSHA256, v.Downloads, v.Yanked, v.ReleaseNotes, v.PublishedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ListVersions lists all versions for a plugin.
func (r *PluginRegistryRepo) ListVersions(ctx context.Context, pluginID uuid.UUID) ([]*models.PluginVersion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, plugin_id, version, manifest_json, checksum_sha256, downloads, yanked, release_notes, published_at
		FROM plugin_versions WHERE plugin_id = $1
		ORDER BY published_at DESC
	`, pluginID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*models.PluginVersion
	for rows.Next() {
		var v models.PluginVersion
		if err := rows.Scan(&v.ID, &v.PluginID, &v.Version, &v.ManifestJSON, &v.ChecksumSHA256, &v.Downloads, &v.Yanked, &v.ReleaseNotes, &v.PublishedAt); err != nil {
			return nil, err
		}
		versions = append(versions, &v)
	}
	return versions, rows.Err()
}

// YankVersion marks a version as yanked.
func (r *PluginRegistryRepo) YankVersion(ctx context.Context, pluginID uuid.UUID, version string) error {
	result, err := r.pool.Exec(ctx, `UPDATE plugin_versions SET yanked = TRUE WHERE plugin_id = $1 AND version = $2`, pluginID, version)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordDownload records a download event.
func (r *PluginRegistryRepo) RecordDownload(ctx context.Context, d *models.PluginDownload) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO plugin_downloads (id, plugin_id, version_id, user_id, ip_hash, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, d.ID, d.PluginID, d.VersionID, d.UserID, d.IPHash, d.CreatedAt)
	return err
}

// IncrementDownloads atomically increments download counters.
func (r *PluginRegistryRepo) IncrementDownloads(ctx context.Context, pluginID, versionID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `UPDATE registry_plugins SET total_downloads = total_downloads + 1 WHERE id = $1`, pluginID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE plugin_versions SET downloads = downloads + 1 WHERE id = $1`, versionID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DownloadStat holds daily download statistics.
type DownloadStat struct {
	Date      time.Time `json:"date"`
	Downloads int64     `json:"downloads"`
}

// GetDownloadStats returns daily download counts for a plugin.
func (r *PluginRegistryRepo) GetDownloadStats(ctx context.Context, pluginID uuid.UUID, days int) ([]DownloadStat, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DATE(created_at) AS date, COUNT(*) AS downloads
		FROM plugin_downloads
		WHERE plugin_id = $1 AND created_at >= NOW() - INTERVAL '1 day' * $2
		GROUP BY DATE(created_at)
		ORDER BY date
	`, pluginID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DownloadStat
	for rows.Next() {
		var s DownloadStat
		if err := rows.Scan(&s.Date, &s.Downloads); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// RecomputeWeeklyDownloads updates weekly_downloads for all plugins.
func (r *PluginRegistryRepo) RecomputeWeeklyDownloads(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE registry_plugins rp
		SET weekly_downloads = COALESCE(sub.cnt, 0)
		FROM (
			SELECT plugin_id, COUNT(*) AS cnt
			FROM plugin_downloads
			WHERE created_at >= NOW() - INTERVAL '7 days'
			GROUP BY plugin_id
		) sub
		WHERE rp.id = sub.plugin_id
	`)
	return err
}

func (r *PluginRegistryRepo) scanPluginsWithPublisher(rows pgx.Rows) ([]*models.RegistryPluginWithPublisher, error) {
	var plugins []*models.RegistryPluginWithPublisher
	for rows.Next() {
		var p models.RegistryPluginWithPublisher
		if err := rows.Scan(
			&p.ID, &p.PublisherID, &p.Name, &p.Description, &p.Type, &p.License, &p.Homepage,
			&p.RepositoryURL, &p.Readme, &p.TotalDownloads, &p.WeeklyDownloads, &p.Stars, &p.Featured,
			&p.Deprecated, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
			&p.PublisherName, &p.PublisherVerified,
		); err != nil {
			return nil, err
		}
		plugins = append(plugins, &p)
	}
	return plugins, rows.Err()
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
