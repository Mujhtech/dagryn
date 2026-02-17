package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/client"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/workflow/ghactions"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new dagryn.toml configuration file",
	Long: `Creates a new dagryn.toml configuration file in the current directory.

By default, dagryn auto-detects your project type based on indicator files
(go.mod, package.json, Cargo.toml, etc.) and generates an appropriate template.

Examples:
  dagryn init                    # Auto-detect project type
  dagryn init --template go      # Force Go template
  dagryn init --interactive      # Interactive project type selector
  dagryn init --list-templates   # Show available templates
  dagryn init --gitignore        # Also add .dagryn/ to .gitignore
  dagryn init --remote           # Create/link project on remote server`,
	RunE: runInit,
}

var (
	forceInit       bool
	interactive     bool
	templateName    string
	listTemplates   bool
	updateGitignore bool
	skipGitignore   bool
	remoteSetup     bool
	projectName     string
)

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "overwrite existing config file")
	initCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactively select project type")
	initCmd.Flags().StringVar(&templateName, "template", "", "force specific template (go, rust, python, node, java, ruby, php, elixir, swift, cpp, generic)")
	initCmd.Flags().BoolVar(&listTemplates, "list-templates", false, "list available templates and exit")
	initCmd.Flags().BoolVar(&updateGitignore, "gitignore", false, "add .dagryn/ to .gitignore")
	initCmd.Flags().BoolVar(&skipGitignore, "no-gitignore", false, "skip .gitignore handling (no warnings)")
	initCmd.Flags().BoolVar(&remoteSetup, "remote", false, "create or link project on remote server")
	initCmd.Flags().StringVar(&projectName, "project-name", "", "project name for remote setup (default: directory name)")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Handle --list-templates
	if listTemplates {
		PrintTemplateList()
		return nil
	}

	projectRoot, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	configPath := filepath.Join(projectRoot, "dagryn.toml")

	// Check if file exists
	if _, err := os.Stat(configPath); err == nil && !forceInit {
		return fmt.Errorf("dagryn.toml already exists. Use --force to overwrite")
	}

	var projectType ProjectType
	var pm PackageManager

	// Priority: --template flag > interactive > auto-detect
	if templateName != "" {
		// Use specified template
		projectType = ProjectTypeFromString(templateName)
		if projectType == "" {
			return fmt.Errorf("unknown template: %q. Use --list-templates to see available options", templateName)
		}
		// For explicitly specified templates, detect package manager if applicable
		result := DetectProject(projectRoot)
		pm = result.PackageManager
	} else {
		// Auto-detect first
		result := DetectProject(projectRoot)

		if interactive {
			// Show interactive selector with detected type as recommended
			selectedType, err := PromptProjectType(result)
			if err != nil {
				return err
			}
			projectType = selectedType
			// Re-detect package manager for the selected type
			if projectType == result.ProjectType {
				pm = result.PackageManager
			} else {
				// User selected different type, use default PM
				pm = ""
			}
		} else {
			// Use auto-detected type
			projectType = result.ProjectType
			pm = result.PackageManager

			// Print detection message
			if projectType != ProjectGeneric {
				fmt.Printf("Detected %s project", projectType.DisplayName())
				if result.IndicatorFile != "" {
					fmt.Printf(" (found %s)", result.IndicatorFile)
				}
				if pm != "" {
					fmt.Printf(" using %s", pm)
				}
				fmt.Println()
			} else {
				fmt.Println("Could not detect project type, using generic template")
				fmt.Println("Tip: Use --template to specify a template, or --interactive to select one")
			}
		}
	}

	// Get template content
	template := GetTemplate(projectType, pm)

	// Write file
	if err := os.WriteFile(configPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("\nCreated %s\n", configPath)

	// If the repo already has GitHub Actions workflows, offer to add hints
	// so users can mirror them as Dagryn tasks.
	if err := maybeSuggestGitHubWorkflows(projectRoot, configPath); err != nil {
		fmt.Printf("Warning: failed to inspect GitHub workflows: %v\n", err)
	}

	// Handle .gitignore
	handleGitignore(projectRoot)

	// Handle remote project setup
	// If --remote flag is set, always do remote setup
	// Otherwise, if user is logged in, ask if they want to set up remote
	shouldSetupRemote := remoteSetup
	if !shouldSetupRemote {
		shouldSetupRemote = shouldPromptForRemoteSetup(projectRoot)
	}

	if shouldSetupRemote {
		if err := setupRemoteProject(projectRoot); err != nil {
			fmt.Printf("\nWarning: Failed to set up remote project: %v\n", err)
			fmt.Println("You can run 'dagryn init --remote' again later to link this project.")
		}
	}

	printNextSteps(projectType)

	return nil
}

// maybeSuggestGitHubWorkflows checks for existing GitHub Actions workflows
// in .github/workflows and, if found, asks the user if they want to translate
// each job/step into Dagryn tasks and append them to dagryn.toml.
func maybeSuggestGitHubWorkflows(projectRoot, configPath string) error {
	workflowsDir := filepath.Join(projectRoot, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		// Directory doesn't exist or can't be read; nothing to do.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var workflowFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			workflowFiles = append(workflowFiles, name)
		}
	}

	if len(workflowFiles) == 0 {
		return nil
	}

	fmt.Println("\nDetected existing GitHub Actions workflows in .github/workflows:")
	for _, f := range workflowFiles {
		fmt.Printf("  - %s\n", f)
	}

	confirm, err := PromptConfirm("Do you want to translate these GitHub workflows into Dagryn tasks now?")
	if err != nil || !confirm {
		return nil
	}

	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var b strings.Builder

	b.WriteString("\n\n")
	files := make(map[string][]byte)

	for _, fname := range workflowFiles {
		path := filepath.Join(workflowsDir, fname)
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: failed to read workflow %s: %v\n", fname, err)
			continue
		}
		files[fname] = content
	}

	translated, err := ghactions.TranslateWorkflows(files)
	if err != nil {
		return err
	}
	if translated.TasksToml == "" {
		return nil
	}
	b.WriteString(translated.TasksToml)

	if _, err := f.WriteString(b.String()); err != nil {
		return err
	}

	fmt.Println("Translated GitHub Actions workflows into Dagryn tasks in dagryn.toml.")
	return nil
}

// sanitizeWorkflowName converts a workflow file name into a safe task prefix.
// func sanitizeWorkflowName(name string) string {
// 	name = strings.ToLower(name)
// 	var b strings.Builder
// 	for _, r := range name {
// 		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
// 			b.WriteRune(r)
// 		} else {
// 			b.WriteRune('_')
// 		}
// 	}
// 	return strings.Trim(b.String(), "_")
// }

// githubWorkflow is a minimal subset of a GitHub Actions workflow used for translation.
// type githubWorkflow struct {
// 	Name string                       `yaml:"name"`
// 	Jobs map[string]githubWorkflowJob `yaml:"jobs"`
// }

// type githubWorkflowJob struct {
// 	Name  string               `yaml:"name"`
// 	Steps []githubWorkflowStep `yaml:"steps"`
// }

// type githubWorkflowStep struct {
// 	Name string `yaml:"name"`
// 	Run  string `yaml:"run"`
// 	Uses string `yaml:"uses"`
// }

// pluginFromUses converts a GitHub Actions "uses:" value into a Dagryn plugin
// key and specification (github:owner/repo@version).
// func pluginFromUses(uses string) (string, string) {
// 	uses = strings.TrimSpace(uses)
// 	if uses == "" {
// 		return "", ""
// 	}

// 	parts := strings.Split(uses, "@")
// 	name := strings.TrimSpace(parts[0])
// 	if name == "" {
// 		return "", ""
// 	}
// 	version := "latest"
// 	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
// 		version = strings.TrimSpace(parts[1])
// 	}

// 	ownerRepo := name
// 	if !strings.Contains(ownerRepo, "/") {
// 		// Common shorthand like "checkout@v4"
// 		ownerRepo = "actions/" + ownerRepo
// 	}

// 	spec := fmt.Sprintf("github:%s@%s", ownerRepo, version)
// 	// Plugin key like actions_checkout
// 	key := sanitizeWorkflowName(strings.ReplaceAll(ownerRepo, "/", "_"))
// 	return key, spec
// }

// escapeForTomlString escapes a string for use inside a double-quoted TOML string.
// func escapeForTomlString(s string) string {
// 	s = strings.ReplaceAll(s, `\`, `\\`)
// 	s = strings.ReplaceAll(s, `"`, `\"`)
// 	s = strings.ReplaceAll(s, "\n", " ")
// 	return s
// }

// handleGitignore handles adding .dagryn/ to .gitignore based on flags
func handleGitignore(projectRoot string) {
	if skipGitignore {
		// User explicitly opted out, do nothing
		return
	}

	// Check if .dagryn is already in .gitignore
	alreadyPresent := gitignoreContainsDagryn(projectRoot)

	if updateGitignore {
		// User explicitly wants to add to gitignore
		if alreadyPresent {
			// Already present, nothing to do
			return
		}

		added, err := addDagrynToGitignore(projectRoot)
		if err != nil {
			fmt.Printf("Warning: Failed to update .gitignore: %v\n", err)
			return
		}
		if added {
			fmt.Println("Added .dagryn/ to .gitignore")
		}
	} else {
		// Default behavior: print a tip if not already in gitignore
		if !alreadyPresent {
			fmt.Println("\nTip: Add .dagryn/ to .gitignore to avoid committing cache files.")
			fmt.Println("     Run 'dagryn init --gitignore' or add it manually.")
		}
	}
}

// gitignoreContainsDagryn checks if .gitignore already has a .dagryn entry
func gitignoreContainsDagryn(projectRoot string) bool {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		// File doesn't exist or can't be read
		return false
	}

	return containsDagrynEntry(string(content))
}

// containsDagrynEntry checks if the content contains a .dagryn gitignore entry
func containsDagrynEntry(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for various patterns: .dagryn, .dagryn/, /.dagryn, /.dagryn/
		if trimmed == ".dagryn" || trimmed == ".dagryn/" ||
			trimmed == "/.dagryn" || trimmed == "/.dagryn/" {
			return true
		}
	}
	return false
}

// addDagrynToGitignore adds .dagryn/ to .gitignore
// Creates .gitignore if it doesn't exist
// Returns true if entry was added, false if already present
func addDagrynToGitignore(projectRoot string) (bool, error) {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	// Read existing content (if file exists)
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read .gitignore: %w", err)
	}

	// Check if already present
	if containsDagrynEntry(string(content)) {
		return false, nil
	}

	// Build new content
	var newContent string
	existingContent := string(content)

	if len(existingContent) == 0 {
		// New file
		newContent = "# Dagryn cache directory\n/.dagryn/\n"
	} else {
		// Append to existing file
		// Ensure there's a newline before our entry
		if !strings.HasSuffix(existingContent, "\n") {
			existingContent += "\n"
		}
		newContent = existingContent + "\n# Dagryn cache directory\n/.dagryn/\n"
	}

	// Write back
	if err := os.WriteFile(gitignorePath, []byte(newContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write .gitignore: %w", err)
	}

	return true, nil
}

// printNextSteps prints helpful next steps after creating the config
func printNextSteps(projectType ProjectType) {
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review and customize dagryn.toml for your project")
	fmt.Println("  2. Run 'dagryn run <task>' to execute a task")
	fmt.Println("  3. Run 'dagryn graph' to visualize the task DAG")
	fmt.Println("  4. Run 'dagryn run' to execute the default workflow")

	// Add project-specific tips
	switch projectType {
	case ProjectPython:
		fmt.Println("\nPython tips:")
		fmt.Println("  - Install ruff for fast linting: pip install ruff")
		fmt.Println("  - Install black for formatting: pip install black")
	case ProjectRust:
		fmt.Println("\nRust tips:")
		fmt.Println("  - Run 'rustup component add clippy' if clippy is not installed")
	case ProjectGeneric:
		fmt.Println("\nGeneric template tips:")
		fmt.Println("  - Replace the TODO placeholder commands with your actual build commands")
		fmt.Println("  - Add appropriate 'inputs' globs for better caching")
	}
}

// shouldPromptForRemoteSetup checks if user is logged in and prompts about remote setup.
// Returns true if user wants to set up remote project.
func shouldPromptForRemoteSetup(projectRoot string) bool {
	// Check if user is logged in
	store, err := client.NewCredentialsStore()
	if err != nil {
		return false
	}

	creds, err := store.Load()
	if err != nil || creds == nil {
		// Not logged in, skip remote setup
		return false
	}

	// Check if already linked to a project
	projectStore := client.NewProjectConfigStore(projectRoot)
	if projectStore.Exists() {
		// Already linked, don't prompt again
		return false
	}

	// User is logged in and project not linked - ask them
	fmt.Printf("\nYou're logged in as %s.\n", creds.UserEmail)
	wantsRemote, err := PromptConfirm("Would you like to link this project to the remote server?")
	if err != nil {
		// User cancelled or error, skip remote setup
		return false
	}

	return wantsRemote
}

// setupRemoteProject handles creating or linking a project on the remote server.
func setupRemoteProject(projectRoot string) error {
	fmt.Println("\nSetting up remote project...")

	// Load credentials
	store, err := client.NewCredentialsStore()
	if err != nil {
		return fmt.Errorf("failed to create credentials store: %w", err)
	}

	creds, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	if creds == nil {
		return fmt.Errorf("not logged in. Run 'dagryn auth login' first")
	}

	// Check for existing project config
	projectStore := client.NewProjectConfigStore(projectRoot)
	existingConfig, err := projectStore.Load()
	if err != nil {
		return fmt.Errorf("failed to check existing project config: %w", err)
	}

	if existingConfig != nil {
		fmt.Printf("This project is already linked to: %s\n", existingConfig.ProjectName)
		relink, err := PromptConfirm("Do you want to re-link to a different project?")
		if err != nil {
			return err
		}
		if !relink {
			fmt.Println("Keeping existing project link.")
			return nil
		}
	}

	// Create API client
	apiClient := client.New(client.Config{
		BaseURL: creds.ServerURL,
		Timeout: 30 * time.Second,
	})

	// Refresh token if expired
	if creds.IsExpired() {
		tokens, err := apiClient.RefreshToken(context.Background(), creds.RefreshToken)
		if err != nil {
			if client.IsNetworkError(err) {
				return fmt.Errorf("cannot connect to server at %s\n\nPlease check your network connection and ensure the server is running", creds.ServerURL)
			}
			return fmt.Errorf("session expired, please login again: %w", err)
		}
		creds.AccessToken = tokens.Data.AccessToken
		creds.RefreshToken = tokens.Data.RefreshToken
		creds.ExpiresAt = tokens.Data.ExpiresAt
		if err := store.Save(creds); err != nil {
			fmt.Printf("Warning: Failed to save refreshed credentials: %v\n", err)
		}
	}

	apiClient.SetCredentials(creds)
	apiClient.SetCredentialsStore(store)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get existing projects
	projects, err := apiClient.ListProjects(ctx)
	if err != nil {
		if client.IsNetworkError(err) {
			return fmt.Errorf("cannot connect to server at %s\n\nPlease check your network connection and ensure the server is running", creds.ServerURL)
		}
		if client.IsAuthError(err) {
			return fmt.Errorf("authentication failed. Please run 'dagryn auth login' to re-authenticate")
		}
		return fmt.Errorf("failed to list projects: %w", err)
	}

	// Convert to selectable projects
	selectableProjects := make([]SelectableProject, 0, len(projects))
	for _, p := range projects {
		selectableProjects = append(selectableProjects, SelectableProject{
			ID:   p.ID.String(),
			Name: p.Name,
			Slug: p.Slug,
			Desc: p.Description,
		})
	}

	// Prompt to select existing or create new
	selectedProject, createNew, err := PromptProjectSelection(selectableProjects)
	if err != nil {
		return err
	}

	var projectConfig *client.ProjectConfig

	if createNew {
		// Create new project
		config, err := createNewProject(ctx, apiClient, projectRoot)
		if err != nil {
			return err
		}
		projectConfig = config
	} else {
		// Use selected existing project
		projID, err := uuid.Parse(selectedProject.ID)
		if err != nil {
			return fmt.Errorf("invalid project ID: %w", err)
		}

		projectConfig = &client.ProjectConfig{
			ProjectID:   projID,
			ServerURL:   creds.ServerURL,
			ProjectName: selectedProject.Name,
			ProjectSlug: selectedProject.Slug,
			LinkedAt:    time.Now(),
		}
	}

	// Save project config
	if err := projectStore.Save(projectConfig); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	fmt.Printf("\nProject linked successfully!\n")
	fmt.Printf("  Name: %s\n", projectConfig.ProjectName)
	fmt.Printf("  ID:   %s\n", projectConfig.ProjectID)

	// Sync workflow to remote after linking
	if err := syncWorkflowToRemote(ctx, apiClient, projectRoot, projectConfig.ProjectID); err != nil {
		fmt.Printf("\nWarning: Failed to sync workflow: %v\n", err)
		fmt.Println("You can sync later using 'dagryn run --sync'.")
	} else {
		fmt.Println("\nWorkflow synced to remote server.")
	}

	fmt.Println("\nYou can now use 'dagryn run --sync' without specifying --project.")

	return nil
}

// getGitRemoteURL attempts to get the remote URL from git.
// It tries 'origin' first, then falls back to any other remote.
func getGitRemoteURL(projectRoot string) string {
	// Try to get 'origin' remote first
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err == nil {
		return normalizeGitURL(strings.TrimSpace(string(output)))
	}

	// Fallback: Get list of remotes and use the first one
	cmd = exec.Command("git", "remote")
	cmd.Dir = projectRoot
	output, err = cmd.Output()
	if err != nil {
		return ""
	}

	remotes := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(remotes) == 0 || remotes[0] == "" {
		return ""
	}

	// Get URL of the first remote
	cmd = exec.Command("git", "remote", "get-url", remotes[0])
	cmd.Dir = projectRoot
	output, err = cmd.Output()
	if err != nil {
		return ""
	}

	return normalizeGitURL(strings.TrimSpace(string(output)))
}

// normalizeGitURL converts SSH git URLs to HTTPS format for consistency.
func normalizeGitURL(url string) string {
	// Convert git@github.com:user/repo.git -> https://github.com/user/repo
	if strings.HasPrefix(url, "git@") {
		// git@github.com:user/repo.git
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
		url = "https://" + url
	}

	// Remove .git suffix for cleaner URLs
	url = strings.TrimSuffix(url, ".git")

	return url
}

// createNewProject handles the interactive creation of a new project on the server.
func createNewProject(ctx context.Context, apiClient *client.Client, projectRoot string) (*client.ProjectConfig, error) {
	// Determine project name
	name := projectName
	if name == "" {
		// Use directory name as default
		name = filepath.Base(projectRoot)
	}

	fmt.Printf("\nCreating new project: %s\n", name)

	// Get teams
	teams, err := apiClient.ListTeams(ctx)
	if err != nil {
		if client.IsNetworkError(err) {
			return nil, fmt.Errorf("cannot connect to server. please check your network connection")
		}
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	// Convert to selectable teams
	selectableTeams := make([]SelectableTeam, 0, len(teams))
	for _, t := range teams {
		selectableTeams = append(selectableTeams, SelectableTeam{
			ID:   t.ID.String(),
			Name: t.Name,
			Slug: t.Slug,
			Desc: t.Description,
		})
	}

	// Prompt for team selection
	var teamID uuid.UUID
	var teamName string
	if len(selectableTeams) > 0 {
		selectedTeam, err := PromptTeamSelection(selectableTeams)
		if err != nil {
			return nil, err
		}
		if !selectedTeam.IsPersonal {
			teamID, err = uuid.Parse(selectedTeam.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid team ID: %w", err)
			}
			teamName = selectedTeam.Name
		}
	}

	// Prompt for visibility
	visibility, err := PromptVisibility()
	if err != nil {
		return nil, err
	}

	// Detect git remote URL
	repoURL := getGitRemoteURL(projectRoot)
	if repoURL != "" {
		fmt.Printf("Detected git remote: %s\n", repoURL)
	}

	// Create the project
	req := client.CreateProjectRequest{
		Name:       name,
		Visibility: visibility,
		RepoURL:    repoURL,
	}
	if teamID != uuid.Nil {
		req.TeamID = teamID
	}

	fmt.Println("\nCreating project on server...")
	project, err := apiClient.CreateProject(ctx, req)
	if err != nil {
		if client.IsNetworkError(err) {
			return nil, fmt.Errorf("cannot connect to server. please check your network connection and try again")
		}
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Get credentials for server URL
	store, _ := client.NewCredentialsStore()
	creds, _ := store.Load()
	serverURL := ""
	if creds != nil {
		serverURL = creds.ServerURL
	}

	return &client.ProjectConfig{
		ProjectID:   project.ID,
		ServerURL:   serverURL,
		ProjectName: project.Name,
		ProjectSlug: project.Slug,
		TeamID:      teamID,
		TeamName:    teamName,
		LinkedAt:    time.Now(),
	}, nil
}

// syncWorkflowToRemote parses dagryn.toml and syncs the workflow to the remote server.
func syncWorkflowToRemote(ctx context.Context, apiClient *client.Client, projectRoot string, projectID uuid.UUID) error {
	configPath := filepath.Join(projectRoot, "dagryn.toml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("dagryn.toml not found")
	}

	// Load and parse the config using config.Parse
	cfg, err := config.Parse(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse dagryn.toml: %w", err)
	}

	// Read raw config for storage
	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Build sync request
	syncReq := client.SyncWorkflowRequest{
		Name:       cfg.Workflow.Name,
		IsDefault:  cfg.Workflow.Default,
		ConfigHash: config.ComputeConfigHash(rawConfig),
		RawConfig:  string(rawConfig),
		Tasks:      make([]client.SyncWorkflowTaskData, 0, len(cfg.Tasks)),
	}

	if syncReq.Name == "" {
		syncReq.Name = "default"
	}

	// Convert tasks
	for name, task := range cfg.Tasks {
		taskData := client.SyncWorkflowTaskData{
			Name:      name,
			Command:   task.Command,
			Needs:     task.Needs,
			Inputs:    task.Inputs,
			Outputs:   task.Outputs,
			Plugins:   task.GetPlugins(),
			Env:       task.Env,
			Group:     task.Group,
			Condition: task.If,
		}
		if task.Workdir != "" {
			taskData.Workdir = &task.Workdir
		}
		syncReq.Tasks = append(syncReq.Tasks, taskData)
	}

	// Sync to server
	resp, err := apiClient.SyncWorkflow(ctx, projectID, syncReq)
	if err != nil {
		return fmt.Errorf("failed to sync workflow: %w", err)
	}

	if resp.Data.Changed {
		fmt.Printf("  Synced: %s (%d tasks)\n", resp.Data.Name, resp.Data.TaskCount)
	}

	return nil
}
