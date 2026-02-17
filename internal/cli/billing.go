package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newBillingCmd())
}

func newBillingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Manage billing and subscription",
		Long:  "Commands for viewing billing status and managing your subscription.",
	}

	cmd.AddCommand(newBillingStatusCmd())
	cmd.AddCommand(newBillingUpgradeCmd())
	cmd.AddCommand(newBillingPortalCmd())

	return cmd
}

func newBillingStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current billing plan and usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBillingStatus()
		},
	}
}

func newBillingPortalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "portal",
		Short: "Open the Stripe billing portal to manage your subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBillingPortal()
		},
	}
}

func newBillingUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade [plan]",
		Short: "Upgrade to a paid plan",
		Long:  "List available plans or upgrade to a specific plan. Opens a Stripe checkout page in your browser.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planSlug := ""
			if len(args) > 0 {
				planSlug = args[0]
			}
			return runBillingUpgrade(planSlug)
		},
	}
}

func runBillingUpgrade(planSlug string) error {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil {
		fmt.Println("You are not logged in. Run 'dagryn auth login' first.")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})
	apiClient.SetCredentials(creds)

	// If no plan specified, list available plans
	if planSlug == "" {
		plans, err := apiClient.ListBillingPlans(ctx)
		if err != nil {
			return fmt.Errorf("failed to list plans: %w", err)
		}

		// Get current plan to highlight it
		overview, err := apiClient.GetBillingOverview(ctx)
		currentSlug := ""
		if err == nil && overview.Data.Plan != nil {
			currentSlug = overview.Data.Plan.Slug
		}

		fmt.Println("Available plans:")
		fmt.Println()
		for _, p := range plans {
			marker := "  "
			if p.Slug == currentSlug {
				marker = "* "
			}
			price := "Free"
			if p.PriceCents > 0 {
				price = fmt.Sprintf("$%.2f/%s", float64(p.PriceCents)/100, p.BillingPeriod)
			}
			fmt.Printf("%s%-12s  %-10s  %s\n", marker, p.Slug, price, p.DisplayName)
		}
		fmt.Println("\n  * = current plan")
		fmt.Println("\nTo upgrade, run: dagryn billing upgrade <plan-slug>")
		return nil
	}

	// Create checkout session for the selected plan
	successURL := creds.ServerURL + "/billing?upgraded=true"
	cancelURL := creds.ServerURL + "/billing"

	url, err := apiClient.CreateCheckoutSession(ctx, planSlug, successURL, cancelURL)
	if err != nil {
		return fmt.Errorf("failed to create checkout session: %w", err)
	}

	fmt.Println("Open this URL to complete your upgrade:")
	fmt.Println(url)

	openBrowser(url)

	return nil
}

func runBillingStatus() error {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil {
		fmt.Println("You are not logged in. Run 'dagryn auth login' first.")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})
	apiClient.SetCredentials(creds)

	overview, err := apiClient.GetBillingOverview(ctx)
	if err != nil {
		return fmt.Errorf("failed to get billing status: %w", err)
	}

	plan := overview.Data.Plan
	if plan == nil {
		fmt.Println("Plan:   Free")
		fmt.Println("No active paid subscription.")
		return nil
	}

	fmt.Printf("Plan:     %s\n", plan.DisplayName)
	fmt.Printf("Price:    $%.2f/%s\n", float64(plan.PriceCents)/100, plan.BillingPeriod)

	// Live resource usage with progress bars
	res := overview.Data.ResourceUsage

	fmt.Println("\nUsage:")

	// Storage
	storageLimit := plan.MaxStorageBytes
	if storageLimit == nil {
		storageLimit = plan.MaxCacheBytes
	}
	if res != nil {
		printUsageLine("Storage", formatBytesHuman(res.TotalStorageBytesUsed), storageLimit, res.TotalStorageBytesUsed, formatBytesHuman)
		fmt.Printf("    Cache:         %s\n", formatBytesHuman(res.CacheBytesUsed))
		fmt.Printf("    Artifacts:     %s\n", formatBytesHuman(res.ArtifactBytesUsed))
	} else {
		printUsageLine("Storage", "0 B", storageLimit, 0, formatBytesHuman)
	}

	// Bandwidth
	if res != nil {
		printUsageLine("Bandwidth", formatBytesHuman(res.BandwidthBytesUsed), plan.MaxBandwidth, res.BandwidthBytesUsed, formatBytesHuman)
	} else {
		printUsageLine("Bandwidth", "0 B", plan.MaxBandwidth, 0, formatBytesHuman)
	}

	// Projects
	var projectsCurrent int
	if res != nil {
		projectsCurrent = res.ProjectsUsed
	}
	var projectsLimit *int64
	if plan.MaxProjects != nil {
		v := int64(*plan.MaxProjects)
		projectsLimit = &v
	}
	printUsageLine("Projects", fmt.Sprintf("%d", projectsCurrent), projectsLimit, int64(projectsCurrent), fmtInt)

	// Team Members
	var membersCurrent int
	if res != nil {
		membersCurrent = res.TeamMembersUsed
	}
	var membersLimit *int64
	if plan.MaxTeamMembers != nil {
		v := int64(*plan.MaxTeamMembers)
		membersLimit = &v
	}
	printUsageLine("Team members", fmt.Sprintf("%d", membersCurrent), membersLimit, int64(membersCurrent), fmtInt)

	// Concurrent runs
	var runsCurrent int
	if res != nil {
		runsCurrent = res.ConcurrentRuns
	}
	var runsLimit *int64
	if plan.MaxConcurrent != nil {
		v := int64(*plan.MaxConcurrent)
		runsLimit = &v
	}
	printUsageLine("Concurrent runs", fmt.Sprintf("%d", runsCurrent), runsLimit, int64(runsCurrent), fmtInt)

	// AI Analyses (monthly)
	if plan.AIEnabled {
		var aiCurrent int
		if res != nil {
			aiCurrent = res.AIAnalysesUsed
		}
		var aiLimit *int64
		if plan.MaxAIAnalysesPerMonth != nil {
			v := int64(*plan.MaxAIAnalysesPerMonth)
			aiLimit = &v
		}
		printUsageLine("AI analyses", fmt.Sprintf("%d", aiCurrent), aiLimit, int64(aiCurrent), fmtInt)

		suggestStatus := "disabled"
		if plan.AISuggestionsEnabled {
			suggestStatus = "enabled"
		}
		fmt.Printf("  %-17s %s\n", "AI suggestions:", suggestStatus)
	} else {
		fmt.Printf("  %-17s %s\n", "AI analyses:", "not included in plan")
	}

	if len(overview.Data.Usage) > 0 {
		fmt.Println("\nCurrent period events:")
		for k, v := range overview.Data.Usage {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}

	return nil
}

func runBillingPortal() error {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil {
		fmt.Println("You are not logged in. Run 'dagryn auth login' first.")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})
	apiClient.SetCredentials(creds)

	url, err := apiClient.GetBillingPortalURL(ctx, creds.ServerURL+"/billing")
	if err != nil {
		return fmt.Errorf("failed to create billing portal session: %w", err)
	}

	fmt.Println("Open this URL to manage your subscription:")
	fmt.Println(url)

	// Try to open in browser
	openBrowser(url)

	return nil
}

// printUsageLine prints a labeled usage line with a progress bar when a limit is set.
//
//	Storage:         1.2 GB / 5.0 GB  [████████░░░░░░░░] 24%
//	Bandwidth:       0 B              Unlimited
func printUsageLine(label, currentStr string, limit *int64, current int64, formatLimit func(int64) string) {
	if limit != nil {
		pct := 0
		if *limit > 0 {
			pct = int(current * 100 / *limit)
			if pct > 100 {
				pct = 100
			}
		}
		bar := progressBar(pct, 16)
		fmt.Printf("  %-17s %s / %s  %s %d%%\n", label+":", currentStr, formatLimit(*limit), bar, pct)
	} else {
		fmt.Printf("  %-17s %-10s         Unlimited\n", label+":", currentStr)
	}
}

// progressBar renders a bar like [████████░░░░░░░░] for the given percentage.
func progressBar(pct, width int) string {
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	bar := make([]byte, 0, 2+width*3) // UTF-8 chars are multi-byte
	bar = append(bar, '[')
	for i := 0; i < width; i++ {
		if i < filled {
			bar = append(bar, "█"...)
		} else {
			bar = append(bar, "░"...)
		}
	}
	bar = append(bar, ']')
	return string(bar)
}

func fmtInt(n int64) string {
	return fmt.Sprintf("%d", n)
}

func formatBytesHuman(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}
