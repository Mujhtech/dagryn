package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/spf13/cobra"
)

var (
	serverURL string
)

// newAuthCmd creates the auth command group.
func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  "Commands for managing authentication with the Dagryn server.",
	}

	authCmd.AddCommand(newLoginCmd())
	authCmd.AddCommand(newLogoutCmd())
	authCmd.AddCommand(newWhoamiCmd())

	return authCmd
}

// newLoginCmd creates the login command.
func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Dagryn server",
		Long: `Authenticate with the Dagryn server using the device code flow.

This command will:
1. Request a device code from the server
2. Display a URL and code for you to enter in your browser
3. Wait for you to authorize the device
4. Save authentication tokens locally

Example:
  dagryn auth login
  dagryn auth login --server https://dagryn.example.com`,
		RunE: runLogin,
	}

	cmd.Flags().StringVar(&serverURL, "server", "http://localhost:9000", "Dagryn server URL")

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create client
	apiClient := client.New(client.Config{
		BaseURL: serverURL,
		Timeout: 30 * time.Second,
	})

	// Request device code
	fmt.Println("Requesting device code...")
	deviceCode, err := apiClient.RequestDeviceCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}

	// Display instructions
	fmt.Println()
	fmt.Println("To authenticate, visit:")
	fmt.Printf("  %s\n", deviceCode.Data.VerificationURI)
	fmt.Println()
	fmt.Println("And enter the code:")
	fmt.Printf("  %s\n", deviceCode.Data.UserCode)
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	// Poll for authorization
	pollInterval := time.Duration(deviceCode.Data.Interval) * time.Second
	if pollInterval < time.Second {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeout := time.After(time.Duration(deviceCode.Data.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("login cancelled")
		case <-timeout:
			return fmt.Errorf("device code expired, please try again")
		case <-ticker.C:
			tokens, pending, err := apiClient.PollDeviceCode(ctx, deviceCode.Data.DeviceCode)
			if err != nil {
				return fmt.Errorf("failed to poll device code: %w", err)
			}

			if pending {
				fmt.Print(".")
				continue
			}

			// Successfully authenticated
			fmt.Println()
			fmt.Println()

			// Save credentials
			store, err := client.NewCredentialsStore()
			if err != nil {
				return fmt.Errorf("failed to create credentials store: %w", err)
			}

			creds := &client.Credentials{
				AccessToken:  tokens.Data.AccessToken,
				RefreshToken: tokens.Data.RefreshToken,
				ExpiresAt:    tokens.Data.ExpiresAt,
				UserID:       tokens.Data.User.ID,
				UserEmail:    tokens.Data.User.Email,
				ServerURL:    serverURL,
			}

			if err := store.Save(creds); err != nil {
				return fmt.Errorf("failed to save credentials: %w", err)
			}

			fmt.Printf("Successfully logged in as %s\n", tokens.Data.User.Email)
			if tokens.Data.User.Name != "" {
				fmt.Printf("Welcome, %s!\n", tokens.Data.User.Name)
			}

			return nil
		}
	}
}

// newLogoutCmd creates the logout command.
func newLogoutCmd() *cobra.Command {
	var revokeAll bool

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from the Dagryn server",
		Long: `Log out from the Dagryn server.

This removes your stored credentials and optionally revokes all tokens.

Example:
  dagryn auth logout
  dagryn auth logout --revoke-all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(revokeAll)
		},
	}

	cmd.Flags().BoolVar(&revokeAll, "revoke-all", false, "Revoke all tokens for this account")

	return cmd
}

func runLogout(revokeAll bool) error {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	if creds == nil {
		fmt.Println("You are not logged in.")
		return nil
	}

	// If revoking, call the server
	if revokeAll {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		apiClient := client.New(client.Config{
			BaseURL: creds.ServerURL,
			Timeout: 30 * time.Second,
		})
		apiClient.SetCredentials(creds)

		if err := apiClient.Logout(ctx, true); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to revoke tokens on server: %v\n", err)
		}
	}

	// Delete local credentials
	if err := store.Delete(); err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	fmt.Println("Successfully logged out.")
	return nil
}

// newWhoamiCmd creates the whoami command.
func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the current authenticated user",
		Long: `Show information about the currently authenticated user.

Example:
  dagryn auth whoami`,
		RunE: runWhoami,
	}
}

func runWhoami(cmd *cobra.Command, args []string) error {
	store, err := client.NewCredentialsStore()
	if err != nil {
		return fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	if creds == nil {
		fmt.Println("You are not logged in.")
		fmt.Println("Run 'dagryn auth login' to authenticate.")
		return nil
	}

	// Create client and get current user
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})

	// Refresh token if expired
	if creds.IsExpired() {
		fmt.Println("Refreshing access token...")
		tokens, err := apiClient.RefreshToken(ctx, creds.RefreshToken)
		if err != nil {
			fmt.Println("Session expired. Please run 'dagryn auth login' again.")
			return nil
		}

		creds.AccessToken = tokens.Data.AccessToken
		creds.RefreshToken = tokens.Data.RefreshToken
		creds.ExpiresAt = tokens.Data.ExpiresAt
		if err := store.Save(creds); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save refreshed credentials: %v\n", err)
		}
	}

	apiClient.SetCredentials(creds)
	apiClient.SetCredentialsStore(store)

	user, err := apiClient.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	fmt.Printf("Logged in as: %s\n", user.Data.Email)
	if user.Data.Name != "" {
		fmt.Printf("Name: %s\n", user.Data.Name)
	}
	fmt.Printf("Provider: %s\n", user.Data.Provider)
	fmt.Printf("Server: %s\n", creds.ServerURL)

	return nil
}
