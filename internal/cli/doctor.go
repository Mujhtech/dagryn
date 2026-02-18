package cli

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/logger"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check your environment for potential issues",
		Long: `Run a series of diagnostic checks to verify your dagryn environment
is set up correctly.

Checks include: configuration validity, Docker/Podman availability,
remote server connectivity, Git version, and plugin health.`,
		Example: `  dagryn doctor`,
		RunE:    runDoctor,
	}
}

func runDoctor(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)
	passed := 0
	warned := 0
	failed := 0

	fmt.Fprintln(cmd.OutOrStdout(), "Dagryn Doctor")
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 40))
	fmt.Fprintln(cmd.OutOrStdout())

	// Check 1: Config file
	cfg, err := config.Parse(cfgFile)
	if err != nil {
		log.Errorf("Config:    %s not found or invalid", cfgFile)
		failed++
	} else {
		errs := config.Validate(cfg)
		if len(errs) > 0 {
			log.Warnf("Config:    %s has %d validation error(s)", cfgFile, len(errs))
			warned++
		} else {
			log.Successf("Config:    %s (%d tasks)", cfgFile, len(cfg.Tasks))
			passed++
		}
	}

	// Check 2: Git
	gitPath, err := exec.LookPath("git")
	if err != nil {
		log.Errorf("Git:       not found in PATH")
		failed++
	} else {
		out, err := exec.Command(gitPath, "--version").Output()
		if err != nil {
			log.Warnf("Git:       found but could not get version")
			warned++
		} else {
			log.Successf("Git:       %s", strings.TrimSpace(string(out)))
			passed++
		}
	}

	// Check 3: Docker / Podman
	dockerOK := false
	dockerPath, err := exec.LookPath("docker")
	if err == nil {
		out, err := exec.Command(dockerPath, "version", "--format", "{{.Server.Version}}").Output()
		if err == nil {
			log.Successf("Docker:    %s", strings.TrimSpace(string(out)))
			dockerOK = true
			passed++
		}
	}
	if !dockerOK {
		podmanPath, err := exec.LookPath("podman")
		if err == nil {
			out, err := exec.Command(podmanPath, "--version").Output()
			if err == nil {
				log.Successf("Podman:    %s", strings.TrimSpace(string(out)))
				passed++
			} else {
				log.Warnf("Container: Podman found but not running")
				warned++
			}
		} else {
			log.Warnf("Container: neither Docker nor Podman found (optional)")
			warned++
		}
	}

	// Check 4: Remote server connectivity
	store, err := client.NewCredentialsStore()
	if err == nil {
		creds, err := store.Load()
		if err == nil && creds != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, "GET", creds.ServerURL+"/api/v1/health", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Errorf("Server:    cannot reach %s", creds.ServerURL)
				failed++
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Successf("Server:    connected to %s", creds.ServerURL)
					passed++
				} else {
					log.Warnf("Server:    %s returned %d", creds.ServerURL, resp.StatusCode)
					warned++
				}
			}
		} else {
			log.Infof("Server:    not logged in (skipped)")
		}
	}

	// Check 5: Plugin health
	if cfg != nil && len(cfg.Plugins) > 0 {
		log.Successf("Plugins:   %d configured", len(cfg.Plugins))
		passed++
	} else if cfg != nil {
		log.Infof("Plugins:   none configured")
	}

	// Summary
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 40))
	log.Infof("Results: %d passed, %d warnings, %d failed", passed, warned, failed)

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}
