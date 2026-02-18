package license

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/cli"
	"github.com/mujhtech/dagryn/pkg/api/handlers"
	"github.com/mujhtech/dagryn/pkg/config"
	"github.com/mujhtech/dagryn/pkg/licensing"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// NewCmd creates the license command.
func NewCmd(_ *cli.Flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "View and manage your Dagryn license",
		Long:  "Commands for viewing license status and activating or deactivating licenses.",
	}

	cmd.AddCommand(newLicenseStatusCmd())
	cmd.AddCommand(newLicenseActivateCmd())
	cmd.AddCommand(newLicenseDeactivateCmd())

	return cmd
}

func newLicenseStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current license status",
		Example: `  dagryn license status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLicenseStatus()
		},
	}
}

func newLicenseActivateCmd() *cobra.Command {
	var instanceName string
	cmd := &cobra.Command{
		Use:   "activate <license-key>",
		Short: "Activate a license key",
		Example: `  dagryn license activate LK-xxxxx-xxxxx
  dagryn license activate LK-xxxxx-xxxxx --name "CI Server"`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLicenseActivate(args[0], instanceName)
		},
	}
	cmd.Flags().StringVar(&instanceName, "name", "", "human-readable name for this instance")
	return cmd
}

func newLicenseDeactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate the license on this instance",
		Example: `  dagryn license deactivate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLicenseDeactivate()
		},
	}
}

func runLicenseStatus() error {
	config.LoadDotEnv()
	key := os.Getenv("DAGRYN_LICENSE_KEY")
	if key == "" {
		// Try to read from stored license
		stored, err := loadStoredLicense()
		if err == nil && stored.Key != "" {
			key = stored.Key
		}
	}

	if key == "" {
		fmt.Println("Dagryn License Status")
		fmt.Println(strings.Repeat("─", 21))
		fmt.Println("Edition:     Community (free)")
		fmt.Println("Licensed:    No")
		fmt.Println()
		fmt.Println("To activate a license, run: dagryn license activate <key>")
		return nil
	}

	keys, err := licensing.ParsePublicKeys()
	if err != nil || len(keys) == 0 {
		return fmt.Errorf("no license verification keys available in this build")
	}

	validator := licensing.NewValidator(keys)
	claims, err := validator.Validate(key)
	if err != nil {
		return fmt.Errorf("invalid license: %w", err)
	}

	gate := licensing.NewFeatureGate(claims, zerolog.Nop())

	fmt.Println("Dagryn License Status")
	fmt.Println(strings.Repeat("─", 21))
	editionStr := string(claims.Edition)
	if len(editionStr) > 0 {
		editionStr = strings.ToUpper(editionStr[:1]) + editionStr[1:]
	}
	fmt.Printf("Edition:     %s\n", editionStr)
	fmt.Printf("Customer:    %s\n", claims.Subject)
	fmt.Printf("License ID:  %s\n", claims.LicenseID)
	fmt.Printf("Seats:       %d\n", claims.Seats)

	// Instance info
	stored, err := loadStoredLicense()
	if err == nil && stored.InstanceID != "" {
		fmt.Printf("Instance:    %s (%s)\n", stored.InstanceName, stored.InstanceID)
	}

	// Features
	fmt.Println("\nFeatures:")
	allFeatures := []struct {
		feature licensing.Feature
		label   string
	}{
		{licensing.FeatureContainerExecution, "Container Execution"},
		{licensing.FeaturePriorityQueue, "Priority Queue"},
		{licensing.FeatureSSO, "SSO / SAML"},
		{licensing.FeatureAuditLogs, "Audit Logs"},
		{licensing.FeatureCustomRBAC, "Custom RBAC"},
		{licensing.FeatureMultiCluster, "Multi-cluster"},
		{licensing.FeatureDashboardFull, "Dashboard (full)"},
		{licensing.FeatureCloudCache, "Cloud Cache"},
	}
	for _, f := range allFeatures {
		if gate.HasFeature(f.feature) {
			fmt.Printf("  ✓ %s\n", f.label)
		} else {
			fmt.Printf("  ✗ %s (not licensed)\n", f.label)
		}
	}

	// Limits
	fmt.Println("\nLimits:")
	printLicenseLimit("Projects", claims.Limits.MaxProjects)
	printLicenseLimit("Team Members", claims.Limits.MaxTeamMembers)
	printLicenseLimit("Concurrent Runs", claims.Limits.MaxConcurrentRuns)

	// Expiry
	fmt.Printf("\nExpiry:  %s (%d days remaining)\n", claims.ExpiryTime().Format("2006-01-02"), claims.DaysUntilExpiry())

	if gate.InGracePeriod() {
		fmt.Println("Status:  ⚠ Grace Period (features will be disabled soon)")
	} else if gate.IsExpiring() {
		fmt.Println("Status:  ⚠ Expiring Soon")
	} else {
		fmt.Println("Status:  ● Active")
	}

	return nil
}

func printLicenseLimit[T int | int64](label string, limit *T) {
	if limit == nil {
		fmt.Printf("  %-17s Unlimited\n", label+":")
	} else {
		fmt.Printf("  %-17s %d\n", label+":", *limit)
	}
}

func runLicenseActivate(key string, instanceName string) error {
	config.LoadDotEnv()
	// 1. Validate locally
	keys, err := licensing.ParsePublicKeys()
	if err != nil || len(keys) == 0 {
		return fmt.Errorf("no license verification keys available in this build")
	}

	validator := licensing.NewValidator(keys)
	claims, err := validator.Validate(key)
	if err != nil {
		return fmt.Errorf("invalid license key: %w", err)
	}
	fmt.Printf("License valid: %s edition for %s\n", claims.Edition, claims.Subject)

	// 2. Load or create instance ID
	stored, _ := loadStoredLicense()
	if stored.InstanceID == "" {
		stored.InstanceID = "inst_" + uuid.New().String()[:8]
	}
	if instanceName != "" {
		stored.InstanceName = instanceName
	} else if stored.InstanceName == "" {
		hostname, _ := os.Hostname()
		stored.InstanceName = hostname
	}

	// 3. Try to register with License Server (non-blocking on failure)
	serverURL := os.Getenv("DAGRYN_LICENSE_SERVER_URL")
	if serverURL == "" {
		serverURL = "https://license.dagryn.dev"
	}

	serverClient := licensing.NewServerClient(licensing.ServerConfig{
		BaseURL: serverURL,
		Timeout: 10 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := serverClient.Activate(ctx, licensing.ActivationRequest{
		LicenseKey:   key,
		InstanceID:   stored.InstanceID,
		InstanceName: stored.InstanceName,
		Version:      handlers.Version,
	})
	if err != nil {
		fmt.Printf("Warning: could not reach License Server (%v)\n", err)
		fmt.Println("Activating in offline mode. License is valid based on local verification.")
	} else if !resp.Activated {
		return fmt.Errorf("activation failed: %s", resp.Message)
	} else {
		fmt.Printf("Registered with License Server: %s\n", resp.Message)
	}

	// 4. Save license locally
	stored.Key = key
	stored.LicenseID = claims.LicenseID
	stored.ActivatedAt = time.Now()
	if err := saveStoredLicense(stored); err != nil {
		return fmt.Errorf("failed to save license: %w", err)
	}

	fmt.Println("\nLicense activated. Restart the server to apply.")
	return nil
}

func runLicenseDeactivate() error {
	config.LoadDotEnv()
	stored, err := loadStoredLicense()
	if err != nil || stored.Key == "" {
		fmt.Println("No license is currently activated.")
		return nil
	}

	// Try to deactivate from License Server
	serverURL := os.Getenv("DAGRYN_LICENSE_SERVER_URL")
	if serverURL == "" {
		serverURL = "https://license.dagryn.dev"
	}

	if stored.LicenseID != "" && stored.InstanceID != "" {
		serverClient := licensing.NewServerClient(licensing.ServerConfig{
			BaseURL: serverURL,
			Timeout: 10 * time.Second,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := serverClient.Deactivate(ctx, licensing.DeactivateRequest{
			LicenseID:  stored.LicenseID,
			InstanceID: stored.InstanceID,
		}); err != nil {
			fmt.Printf("Warning: could not reach License Server (%v)\n", err)
			fmt.Println("Removing local license anyway.")
		} else {
			fmt.Println("Deactivated from License Server.")
		}
	}

	// Remove local license
	if err := removeStoredLicense(); err != nil {
		return fmt.Errorf("failed to remove stored license: %w", err)
	}

	fmt.Println("License deactivated. This instance is now running as Community edition.")
	fmt.Println("Restart the server to apply.")
	return nil
}

// storedLicense holds local license data persisted in ~/.dagryn/license.json.
type storedLicense struct {
	Key          string    `json:"key,omitempty"`
	LicenseID    string    `json:"license_id,omitempty"`
	InstanceID   string    `json:"instance_id,omitempty"`
	InstanceName string    `json:"instance_name,omitempty"`
	ActivatedAt  time.Time `json:"activated_at,omitempty"`
}

func licensePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".dagryn", "license.json"), nil
}

func loadStoredLicense() (storedLicense, error) {
	var stored storedLicense
	path, err := licensePath()
	if err != nil {
		return stored, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return stored, err
	}
	err = json.Unmarshal(data, &stored)
	return stored, err
}

func saveStoredLicense(stored storedLicense) error {
	path, err := licensePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func removeStoredLicense() error {
	path, err := licensePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
