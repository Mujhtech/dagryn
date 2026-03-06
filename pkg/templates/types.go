package templates

// ProjectType represents the detected project type
type ProjectType string

const (
	ProjectGo      ProjectType = "go"
	ProjectRust    ProjectType = "rust"
	ProjectPython  ProjectType = "python"
	ProjectNode    ProjectType = "node"
	ProjectJava    ProjectType = "java"
	ProjectRuby    ProjectType = "ruby"
	ProjectPHP     ProjectType = "php"
	ProjectElixir  ProjectType = "elixir"
	ProjectSwift   ProjectType = "swift"
	ProjectCpp     ProjectType = "cpp"
	ProjectGeneric ProjectType = "generic"
)

// PackageManager represents the detected package manager
type PackageManager string

// Node.js package managers
const (
	PMNpm  PackageManager = "npm"
	PMYarn PackageManager = "yarn"
	PMPnpm PackageManager = "pnpm"
)

// Python package managers
const (
	PMPip    PackageManager = "pip"
	PMPoetry PackageManager = "poetry"
	PMPipenv PackageManager = "pipenv"
	PMUv     PackageManager = "uv"
)

// Java build tools
const (
	PMMaven  PackageManager = "maven"
	PMGradle PackageManager = "gradle"
)

// C/C++ build systems
const (
	PMCMake PackageManager = "cmake"
	PMMeson PackageManager = "meson"
	PMMake  PackageManager = "make"
)

// ProjectTypeFromString converts a string to ProjectType
func ProjectTypeFromString(s string) ProjectType {
	switch s {
	case "go":
		return ProjectGo
	case "rust":
		return ProjectRust
	case "python":
		return ProjectPython
	case "node", "nodejs", "js", "javascript":
		return ProjectNode
	case "java":
		return ProjectJava
	case "ruby":
		return ProjectRuby
	case "php":
		return ProjectPHP
	case "elixir":
		return ProjectElixir
	case "swift":
		return ProjectSwift
	case "cpp", "c++", "c":
		return ProjectCpp
	case "generic":
		return ProjectGeneric
	default:
		return ""
	}
}

// ProjectTypeFromGitHubLanguage maps a GitHub API language name to a ProjectType.
func ProjectTypeFromGitHubLanguage(lang string) ProjectType {
	switch lang {
	case "Go":
		return ProjectGo
	case "Rust":
		return ProjectRust
	case "Python":
		return ProjectPython
	case "JavaScript", "TypeScript", "Vue", "Svelte":
		return ProjectNode
	case "Java", "Kotlin":
		return ProjectJava
	case "Ruby":
		return ProjectRuby
	case "PHP":
		return ProjectPHP
	case "Elixir":
		return ProjectElixir
	case "Swift":
		return ProjectSwift
	case "C", "C++", "Objective-C":
		return ProjectCpp
	default:
		return ProjectGeneric
	}
}

// String returns the string representation of ProjectType
func (p ProjectType) String() string {
	return string(p)
}

// DisplayName returns a human-friendly name for the project type
func (p ProjectType) DisplayName() string {
	switch p {
	case ProjectGo:
		return "Go"
	case ProjectRust:
		return "Rust"
	case ProjectPython:
		return "Python"
	case ProjectNode:
		return "Node.js"
	case ProjectJava:
		return "Java"
	case ProjectRuby:
		return "Ruby"
	case ProjectPHP:
		return "PHP"
	case ProjectElixir:
		return "Elixir"
	case ProjectSwift:
		return "Swift"
	case ProjectCpp:
		return "C/C++"
	case ProjectGeneric:
		return "Generic"
	default:
		return string(p)
	}
}

// Description returns a brief description of the project type
func (p ProjectType) Description() string {
	switch p {
	case ProjectGo:
		return "Go projects with go build, go test, golangci-lint"
	case ProjectRust:
		return "Rust projects with Cargo"
	case ProjectPython:
		return "Python projects with pip/poetry/pipenv/uv"
	case ProjectNode:
		return "Node.js projects with npm/yarn/pnpm"
	case ProjectJava:
		return "Java projects with Maven or Gradle"
	case ProjectRuby:
		return "Ruby projects with Bundler"
	case ProjectPHP:
		return "PHP projects with Composer"
	case ProjectElixir:
		return "Elixir projects with Mix"
	case ProjectSwift:
		return "Swift projects with Swift Package Manager"
	case ProjectCpp:
		return "C/C++ projects with CMake or Meson"
	case ProjectGeneric:
		return "Generic template with placeholder tasks"
	default:
		return ""
	}
}

// AllProjectTypes returns all available project types
func AllProjectTypes() []ProjectType {
	return []ProjectType{
		ProjectGo,
		ProjectRust,
		ProjectPython,
		ProjectNode,
		ProjectJava,
		ProjectRuby,
		ProjectPHP,
		ProjectElixir,
		ProjectSwift,
		ProjectCpp,
		ProjectGeneric,
	}
}

// TemplateInfo returns template name and description for listing
type TemplateInfo struct {
	Name        string
	Description string
}

// GetAllTemplateInfos returns info about all available templates
func GetAllTemplateInfos() []TemplateInfo {
	return []TemplateInfo{
		{"go", "Go projects with go build, go test, golangci-lint"},
		{"rust", "Rust projects with Cargo"},
		{"python", "Python projects with pip/poetry/pipenv/uv"},
		{"node", "Node.js projects with npm/yarn/pnpm"},
		{"java", "Java projects with Maven or Gradle"},
		{"ruby", "Ruby projects with Bundler"},
		{"php", "PHP projects with Composer"},
		{"elixir", "Elixir projects with Mix"},
		{"swift", "Swift projects with Swift Package Manager"},
		{"cpp", "C/C++ projects with CMake or Meson"},
		{"generic", "Generic template with placeholder tasks"},
	}
}
