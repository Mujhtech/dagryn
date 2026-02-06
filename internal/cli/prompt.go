package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// projectItem represents a selectable project type in the list
type projectItem struct {
	projectType ProjectType
	recommended bool
}

func (i projectItem) Title() string {
	name := i.projectType.DisplayName()
	if i.recommended {
		return name + " (Recommended)"
	}
	return name
}

func (i projectItem) Description() string {
	return i.projectType.Description()
}

func (i projectItem) FilterValue() string {
	return i.projectType.DisplayName()
}

// selectModel is the bubbletea model for project type selection
type selectModel struct {
	list     list.Model
	choice   ProjectType
	quitting bool
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(projectItem); ok {
				m.choice = item.projectType
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectModel) View() string {
	if m.choice != "" {
		return ""
	}
	if m.quitting {
		return ""
	}
	return m.list.View()
}

// itemDelegate handles rendering of list items
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 2 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(projectItem)
	if !ok {
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedTitleStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("170"))
	descStyle := lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	selectedDescStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("245"))

	title := item.Title()
	desc := item.Description()

	if index == m.Index() {
		// Selected item
		fmt.Fprintf(w, "%s\n%s\n",
			selectedTitleStyle.Render("> "+title),
			selectedDescStyle.Render("  "+desc))
	} else {
		// Normal item
		fmt.Fprintf(w, "%s\n%s\n",
			titleStyle.Render(title),
			descStyle.Render(desc))
	}
}

// PromptProjectType shows an interactive selector and returns the chosen project type.
// If detected is provided and has a known project type, that option will be marked as recommended.
func PromptProjectType(detected *DetectionResult) (ProjectType, error) {
	// Build list items
	items := make([]list.Item, 0, len(AllProjectTypes()))

	allTypes := AllProjectTypes()
	for _, pt := range allTypes {
		recommended := false
		if detected != nil && detected.ProjectType == pt && pt != ProjectGeneric {
			recommended = true
		}
		items = append(items, projectItem{
			projectType: pt,
			recommended: recommended,
		})
	}

	// If we have a detected type, move it to the top
	if detected != nil && detected.ProjectType != ProjectGeneric {
		for i, item := range items {
			if pi, ok := item.(projectItem); ok && pi.recommended {
				// Move to front
				items = append([]list.Item{items[i]}, append(items[:i], items[i+1:]...)...)
				break
			}
		}
	}

	// Create list
	const listHeight = 14
	const listWidth = 60

	l := list.New(items, itemDelegate{}, listWidth, listHeight)
	l.Title = "Select project type"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(0).Bold(true)
	l.Styles.HelpStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("241"))

	// Custom help
	l.SetShowHelp(true)

	m := selectModel{list: l}

	// Run the program
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running interactive prompt: %w", err)
	}

	result := finalModel.(selectModel)
	if result.quitting && result.choice == "" {
		return "", fmt.Errorf("selection cancelled")
	}

	return result.choice, nil
}

// PrintTemplateList prints all available templates in a formatted list
func PrintTemplateList() {
	fmt.Println("Available templates:")
	fmt.Println()

	infos := GetAllTemplateInfos()

	// Find max name length for alignment
	maxLen := 0
	for _, info := range infos {
		if len(info.Name) > maxLen {
			maxLen = len(info.Name)
		}
	}

	for _, info := range infos {
		padding := strings.Repeat(" ", maxLen-len(info.Name)+2)
		fmt.Printf("  %s%s%s\n", info.Name, padding, info.Description)
	}
	fmt.Println()
	fmt.Println("Usage: dagryn init --template <name>")
}

// --- Project Selection Types ---

// SelectableProject represents a project that can be selected in the list
type SelectableProject struct {
	ID   string
	Name string
	Slug string
	Desc string
}

func (p SelectableProject) Title() string       { return p.Name }
func (p SelectableProject) Description() string { return p.Desc }
func (p SelectableProject) FilterValue() string { return p.Name }

// SelectableTeam represents a team that can be selected in the list
type SelectableTeam struct {
	ID         string
	Name       string
	Slug       string
	Desc       string
	IsPersonal bool
}

func (t SelectableTeam) Title() string {
	if t.IsPersonal {
		return t.Name + " (Personal)"
	}
	return t.Name
}
func (t SelectableTeam) Description() string { return t.Desc }
func (t SelectableTeam) FilterValue() string { return t.Name }

// VisibilityOption represents a visibility option
type VisibilityOption struct {
	Value string
	Label string
	Desc  string
}

func (v VisibilityOption) Title() string       { return v.Label }
func (v VisibilityOption) Description() string { return v.Desc }
func (v VisibilityOption) FilterValue() string { return v.Label }

// genericItemDelegate handles rendering of list items with title only
type genericItemDelegate struct{}

func (d genericItemDelegate) Height() int                             { return 1 }
func (d genericItemDelegate) Spacing() int                            { return 0 }
func (d genericItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d genericItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	title := listItem.(list.DefaultItem).Title()

	titleStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedTitleStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("170"))

	if index == m.Index() {
		fmt.Fprintf(w, "%s\n", selectedTitleStyle.Render("> "+title))
	} else {
		fmt.Fprintf(w, "%s\n", titleStyle.Render(title))
	}
}

// selectableItemDelegate handles rendering of selectable project/team items
type selectableItemDelegate struct{}

func (d selectableItemDelegate) Height() int                             { return 2 }
func (d selectableItemDelegate) Spacing() int                            { return 0 }
func (d selectableItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d selectableItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(list.DefaultItem)
	if !ok {
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedTitleStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("170"))
	descStyle := lipgloss.NewStyle().PaddingLeft(4).Foreground(lipgloss.Color("241"))
	selectedDescStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("245"))

	title := item.Title()
	desc := item.Description()

	if index == m.Index() {
		// Selected item
		fmt.Fprintf(w, "%s\n%s\n",
			selectedTitleStyle.Render("> "+title),
			selectedDescStyle.Render("  "+desc))
	} else {
		// Normal item
		fmt.Fprintf(w, "%s\n%s\n",
			titleStyle.Render(title),
			descStyle.Render(desc))
	}
}

// genericSelectModel is a generic bubbletea model for selections
type genericSelectModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m genericSelectModel) Init() tea.Cmd { return nil }

func (m genericSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(list.DefaultItem); ok {
				m.choice = item.FilterValue()
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m genericSelectModel) View() string {
	if m.choice != "" || m.quitting {
		return ""
	}
	return m.list.View()
}

// PromptProjectSelection shows an interactive selector for projects.
// Returns the selected project, or nil if "Create new project" is selected.
func PromptProjectSelection(projects []SelectableProject) (*SelectableProject, bool, error) {
	// Build list items - "Create new project" first
	items := make([]list.Item, 0, len(projects)+1)
	items = append(items, list.Item(SelectableProject{
		ID:   "__new__",
		Name: "Create new project",
		Desc: "Create a new project on the server",
	}))

	for _, p := range projects {
		items = append(items, list.Item(p))
	}

	const listHeight = 10
	const listWidth = 60

	l := list.New(items, selectableItemDelegate{}, listWidth, listHeight)
	l.Title = "Select a project"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(0).Bold(true)
	l.Styles.HelpStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("241"))

	m := selectModel{list: l}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, false, fmt.Errorf("error running interactive prompt: %w", err)
	}

	result := finalModel.(selectModel)
	if result.quitting && result.choice == "" {
		return nil, false, fmt.Errorf("selection cancelled")
	}

	// Find the selected project
	for _, proj := range projects {
		if proj.Name == string(result.choice) {
			return &proj, false, nil
		}
	}

	// "Create new project" was selected
	return nil, true, nil
}

// PromptTeamSelection shows an interactive selector for teams.
// Returns the selected team ID (empty UUID for personal project).
func PromptTeamSelection(teams []SelectableTeam) (*SelectableTeam, error) {
	// Build list items - "Personal" first
	items := make([]list.Item, 0, len(teams)+1)
	items = append(items, list.Item(SelectableTeam{
		ID:         "",
		Name:       "Personal",
		Desc:       "Create as a personal project (no team)",
		IsPersonal: true,
	}))

	for _, t := range teams {
		items = append(items, list.Item(t))
	}

	const listHeight = 10
	const listWidth = 60

	l := list.New(items, selectableItemDelegate{}, listWidth, listHeight)
	l.Title = "Select a team (or personal)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(0).Bold(true)
	l.Styles.HelpStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("241"))

	m := selectModel{list: l}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running interactive prompt: %w", err)
	}

	result := finalModel.(selectModel)
	if result.quitting && result.choice == "" {
		return nil, fmt.Errorf("selection cancelled")
	}

	// Find the selected team
	for _, team := range teams {
		if team.Name == string(result.choice) {
			return &team, nil
		}
	}

	// Personal was selected
	return &SelectableTeam{IsPersonal: true, Name: "Personal"}, nil
}

// PromptVisibility shows an interactive selector for project visibility.
// Returns the visibility value (private, team, public).
func PromptVisibility() (string, error) {
	options := []VisibilityOption{
		{Value: "private", Label: "Private (Recommended)", Desc: "Only you and invited members can access"},
		{Value: "team", Label: "Team", Desc: "All team members can access"},
		{Value: "public", Label: "Public", Desc: "Anyone can view (read-only)"},
	}

	items := make([]list.Item, 0, len(options))
	for _, opt := range options {
		items = append(items, list.Item(opt))
	}

	const listHeight = 8
	const listWidth = 60

	l := list.New(items, genericItemDelegate{}, listWidth, listHeight)
	l.Title = "Select project visibility"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(0).Bold(true)
	l.Styles.HelpStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("241"))

	m := genericSelectModel{list: l}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running interactive prompt: %w", err)
	}

	result := finalModel.(genericSelectModel)
	if result.quitting && result.choice == "" {
		return "", fmt.Errorf("selection cancelled")
	}

	// Find the selected visibility
	for _, opt := range options {
		if opt.Label == result.choice {
			return opt.Value, nil
		}
	}

	return "private", nil
}

// PromptConfirm shows a yes/no confirmation prompt.
func PromptConfirm(message string) (bool, error) {
	options := []VisibilityOption{
		{Value: "yes", Label: "Yes"},
		{Value: "no", Label: "No"},
	}

	items := make([]list.Item, 0, len(options))
	for _, opt := range options {
		items = append(items, list.Item(opt))
	}

	const listHeight = 5
	const listWidth = 40

	l := list.New(items, genericItemDelegate{}, listWidth, listHeight)
	l.Title = message
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(0).Bold(true)

	m := genericSelectModel{list: l}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("error running interactive prompt: %w", err)
	}

	result := finalModel.(genericSelectModel)
	if result.quitting && result.choice == "" {
		return false, nil
	}

	return result.choice == "Yes", nil
}
