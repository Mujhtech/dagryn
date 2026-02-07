package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mujhtech/dagryn/internal/cache"
	"github.com/mujhtech/dagryn/internal/cache/remote"
	"github.com/mujhtech/dagryn/internal/config"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/mujhtech/dagryn/pkg/storage"
	"github.com/spf13/cobra"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the build cache",
	}
	cmd.AddCommand(newCacheStatusCmd())
	cmd.AddCommand(newCacheClearCmd())
	cmd.AddCommand(newCachePushCmd())
	cmd.AddCommand(newCachePullCmd())
	return cmd
}

func newCacheStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cache status and remote connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(verbose)
			projectRoot, err := getProjectRoot()
			if err != nil {
				return err
			}

			cfg, err := config.Parse(cfgFile)
			if err != nil {
				return err
			}

			store := cache.NewStore(projectRoot)
			log.Info(fmt.Sprintf("Project root: %s", projectRoot))
			log.Info(fmt.Sprintf("Local cache:  %s", store.CachePath()))

			entries, err := store.ListEntries("")
			if err != nil {
				log.Info(fmt.Sprintf("Local cache:  error listing - %v", err))
			} else {
				log.Info(fmt.Sprintf("Local cache:  %d entries", len(entries)))
			}

			if cfg.Cache.Remote.Enabled {
				log.Info(fmt.Sprintf("Remote cache: enabled (provider=%s)", cfg.Cache.Remote.Provider))
				bucket, err := buildBucket(cfg.Cache.Remote)
				if err != nil {
					log.Info(fmt.Sprintf("Remote cache: connection FAILED - %v", err))
				} else {
					_, err := bucket.List(context.Background(), "", &storage.ListOptions{MaxKeys: 1})
					if err != nil {
						log.Info(fmt.Sprintf("Remote cache: connection FAILED - %v", err))
					} else {
						log.Info("Remote cache: connected")
					}
				}
			} else {
				log.Info("Remote cache: disabled")
			}
			return nil
		},
	}
}

func newCacheClearCmd() *cobra.Command {
	var taskName string
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the local cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(verbose)
			projectRoot, err := getProjectRoot()
			if err != nil {
				return err
			}
			c := cache.New(projectRoot, true)
			ctx := context.Background()
			if taskName != "" {
				if err := c.Clear(ctx, taskName); err != nil {
					return err
				}
				log.Info(fmt.Sprintf("Cleared cache for task %q", taskName))
			} else {
				if err := c.ClearAll(ctx); err != nil {
					return err
				}
				log.Info("Cleared all local cache")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&taskName, "task", "", "clear cache for a specific task only")
	return cmd
}

func newCachePushCmd() *cobra.Command {
	var taskName string
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push local cache to remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(verbose)
			projectRoot, err := getProjectRoot()
			if err != nil {
				return err
			}

			cfg, err := config.Parse(cfgFile)
			if err != nil {
				return err
			}

			if !cfg.Cache.Remote.Enabled {
				return fmt.Errorf("remote cache is not enabled in config")
			}

			bucket, err := buildBucket(cfg.Cache.Remote)
			if err != nil {
				return fmt.Errorf("failed to connect to remote cache: %w", err)
			}

			store := cache.NewStore(projectRoot)
			remoteBackend := remote.NewStorageBackend(bucket, projectRoot)
			ctx := context.Background()

			entries, err := store.ListEntries(taskName)
			if err != nil {
				return fmt.Errorf("failed to list local cache: %w", err)
			}

			if len(entries) == 0 {
				log.Info("No local cache entries to push")
				return nil
			}

			log.Info(fmt.Sprintf("Pushing %d cache entries to remote (provider=%s)...", len(entries), cfg.Cache.Remote.Provider))

			pushed := 0
			for _, entry := range entries {
				// Read metadata to get output file list
				meta, err := store.GetMetadata(entry.TaskName, entry.CacheKey)
				if err != nil {
					log.Error(fmt.Sprintf("skip %s/%s: failed to read metadata", entry.TaskName, entry.CacheKey), err)
					continue
				}

				// Check if already in remote
				exists, err := remoteBackend.Check(ctx, entry.TaskName, entry.CacheKey)
				if err == nil && exists {
					if verbose {
						log.Info(fmt.Sprintf("  skip %s/%s (already in remote)", entry.TaskName, entry.CacheKey))
					}
					continue
				}

				// Build output patterns from stored outputs:
				// The outputs are stored as relative paths under the outputs dir.
				// We need to temporarily restore them to project root so the remote
				// backend can read them. Instead, we'll read directly from the
				// cache outputs directory.
				outputsDir := store.OutputsPath(entry.TaskName, entry.CacheKey)
				var outputFiles []string
				_ = filepath.Walk(outputsDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(outputsDir, path)
					if err != nil {
						return nil
					}
					outputFiles = append(outputFiles, rel)
					return nil
				})

				// Copy cached outputs to project root temporarily so the remote
				// backend can find and upload them
				var restored []string
				for _, relPath := range outputFiles {
					src := filepath.Join(outputsDir, relPath)
					dest := filepath.Join(projectRoot, relPath)
					// Only copy if the file doesn't already exist at destination
					if _, err := os.Stat(dest); os.IsNotExist(err) {
						if err := copyFileForPush(src, dest); err == nil {
							restored = append(restored, relPath)
						}
					}
				}

				// Use the stored output list as patterns
				patterns := meta.Outputs
				if len(patterns) == 0 {
					patterns = outputFiles
				}

				if err := remoteBackend.Save(ctx, entry.TaskName, entry.CacheKey, patterns, *meta); err != nil {
					log.Error(fmt.Sprintf("failed to push %s/%s", entry.TaskName, entry.CacheKey), err)
					// Clean up temporarily restored files
					for _, relPath := range restored {
						_ = os.Remove(filepath.Join(projectRoot, relPath))
					}
					continue
				}

				// Clean up temporarily restored files
				for _, relPath := range restored {
					_ = os.Remove(filepath.Join(projectRoot, relPath))
				}

				pushed++
				if verbose {
					log.Info(fmt.Sprintf("  pushed %s/%s (%d files)", entry.TaskName, entry.CacheKey, len(outputFiles)))
				}
			}

			log.Info(fmt.Sprintf("Pushed %d/%d cache entries", pushed, len(entries)))
			return nil
		},
	}
	cmd.Flags().StringVar(&taskName, "task", "", "push cache for a specific task only")
	return cmd
}

func newCachePullCmd() *cobra.Command {
	var taskName string
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull remote cache to local",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.New(verbose)
			projectRoot, err := getProjectRoot()
			if err != nil {
				return err
			}

			cfg, err := config.Parse(cfgFile)
			if err != nil {
				return err
			}

			if !cfg.Cache.Remote.Enabled {
				return fmt.Errorf("remote cache is not enabled in config")
			}

			bucket, err := buildBucket(cfg.Cache.Remote)
			if err != nil {
				return fmt.Errorf("failed to connect to remote cache: %w", err)
			}

			ctx := context.Background()
			store := cache.NewStore(projectRoot)
			remoteBackend := remote.NewStorageBackend(bucket, projectRoot)

			// List action cache entries in remote
			prefix := "ac/"
			if taskName != "" {
				prefix = fmt.Sprintf("ac/%s/", taskName)
			}

			result, err := bucket.List(ctx, prefix, nil)
			if err != nil {
				return fmt.Errorf("failed to list remote cache: %w", err)
			}

			if len(result.Keys) == 0 {
				log.Info("No remote cache entries to pull")
				return nil
			}

			log.Info(fmt.Sprintf("Found %d remote cache entries (provider=%s)...", len(result.Keys), cfg.Cache.Remote.Provider))

			pulled := 0
			for _, key := range result.Keys {
				// Parse task name and cache key from the action key: ac/{taskName}/{cacheKey}
				parts := strings.SplitN(key, "/", 3)
				if len(parts) != 3 || parts[0] != "ac" {
					continue
				}
				entryTask := parts[1]
				entryKey := parts[2]

				// Skip if already in local cache
				if store.Exists(entryTask, entryKey) {
					if verbose {
						log.Info(fmt.Sprintf("  skip %s/%s (already local)", entryTask, entryKey))
					}
					continue
				}

				// Restore from remote (puts files on disk)
				if err := remoteBackend.Restore(ctx, entryTask, entryKey); err != nil {
					log.Error(fmt.Sprintf("failed to pull %s/%s", entryTask, entryKey), err)
					continue
				}

				// Save to local cache so future local checks hit
				meta := cache.Metadata{
					TaskName: entryTask,
					CacheKey: entryKey,
				}
				if err := store.Save(entryTask, entryKey, nil, meta); err != nil {
					log.Error(fmt.Sprintf("failed to save local entry %s/%s", entryTask, entryKey), err)
					continue
				}

				pulled++
				if verbose {
					log.Info(fmt.Sprintf("  pulled %s/%s", entryTask, entryKey))
				}
			}

			log.Info(fmt.Sprintf("Pulled %d/%d cache entries", pulled, len(result.Keys)))
			return nil
		},
	}
	cmd.Flags().StringVar(&taskName, "task", "", "pull cache for a specific task only")
	return cmd
}

// buildBucket creates a storage.Bucket from the remote cache config.
func buildBucket(rc config.RemoteCacheConfig) (storage.Bucket, error) {
	return storage.NewBucket(storage.Config{
		Provider:        storage.ProviderType(rc.Provider),
		BasePath:        rc.BasePath,
		Bucket:          rc.Bucket,
		Region:          rc.Region,
		Endpoint:        rc.Endpoint,
		AccessKeyID:     rc.AccessKeyID,
		SecretAccessKey: rc.SecretAccessKey,
		UsePathStyle:    rc.UsePathStyle,
		Prefix:          rc.Prefix,
	})
}

// copyFileForPush copies a single file, creating parent directories as needed.
func copyFileForPush(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}
