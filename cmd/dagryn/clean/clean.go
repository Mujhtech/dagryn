package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/cliui"
	"github.com/spf13/cobra"
)

const dagrynDir = ".dagryn"

// paths removed by default (relative to .dagryn/).
var defaultCleanPaths = []string{
	"cache",
	"plugins",
	"runs",
	"plugins.lock",
}

// paths preserved unless --force is used.
var preservedPaths = []string{
	"project.json",
	"context.json",
}

// NewCmd creates the clean command.
func NewCmd(flags *cli.Flags) *cobra.Command {
	var force bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove local dagryn cache, plugins, and run data",
		Long: `Remove the .dagryn/ directory contents including cache, plugins,
runs, and the plugins.lock file.

By default, project.json is preserved so your project link remains
intact. Use --force to remove everything including project.json.`,
		Example: `  dagryn clean
  dagryn clean --dry-run
  dagryn clean --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(flags, force, dryRun)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "remove everything including project.json")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without deleting")

	return cmd
}

func runClean(flags *cli.Flags, force, dryRun bool) error {
	w := cliui.NewWriter()

	root, err := cli.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	dagrynPath := filepath.Join(root, dagrynDir)
	if _, err := os.Stat(dagrynPath); os.IsNotExist(err) {
		w.Infof("Nothing to clean — %s does not exist", dagrynDir)
		return nil
	}

	if force {
		return cleanForce(w, dagrynPath, dryRun)
	}

	return cleanDefault(w, dagrynPath, dryRun)
}

// cleanDefault removes only the known ephemeral paths, preserving project.json etc.
func cleanDefault(w *cliui.Writer, dagrynPath string, dryRun bool) error {
	var removed int

	for _, rel := range defaultCleanPaths {
		target := filepath.Join(dagrynPath, rel)
		info, err := os.Stat(target)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			w.Warnf("skip %s: %v", rel, err)
			continue
		}

		size := ""
		if info.IsDir() {
			size = fmt.Sprintf(" (%s)", dirSize(target))
		}

		if dryRun {
			w.Infof("  would remove %s%s", rel, size)
		} else {
			sp := cliui.NewSpinner(os.Stderr, fmt.Sprintf("Removing %s%s", rel, size))
			sp.Start()
			if err := os.RemoveAll(target); err != nil {
				sp.Stop("")
				w.Errorf("failed to remove %s: %v", rel, err)
				continue
			}
			sp.Stop("")
			w.Successf("removed %s%s", rel, size)
		}
		removed++
	}

	if removed == 0 {
		w.Infof("Nothing to clean")
	} else if dryRun {
		w.Infof("\nDry run — no files were removed")
	} else {
		w.Infof("\nPreserved: %s", strings.Join(preservedPaths, ", "))
	}

	return nil
}

// cleanForce removes the entire .dagryn/ directory.
func cleanForce(w *cliui.Writer, dagrynPath string, dryRun bool) error {
	size := dirSize(dagrynPath)

	if dryRun {
		w.Infof("  would remove entire %s/ (%s)", dagrynDir, size)
		w.Infof("\nDry run — no files were removed")
		return nil
	}

	sp := cliui.NewSpinner(os.Stderr, fmt.Sprintf("Removing %s/ (%s)", dagrynDir, size))
	sp.Start()

	if err := os.RemoveAll(dagrynPath); err != nil {
		sp.Stop("")
		return fmt.Errorf("failed to remove %s: %w", dagrynDir, err)
	}

	sp.Stop("")
	w.Successf("removed %s/ (%s)", dagrynDir, size)
	return nil
}

// dirSize walks a directory and returns a human-readable total size.
func dirSize(path string) string {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return humanSize(total)
}

func humanSize(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
