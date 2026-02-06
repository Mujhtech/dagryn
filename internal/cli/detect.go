package cli

import (
	"os"
	"path/filepath"
)

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

// DetectionResult holds the result of project detection
type DetectionResult struct {
	ProjectType    ProjectType
	PackageManager PackageManager
	Confidence     string // "high", "medium", "low"
	IndicatorFile  string // The file that triggered detection
}

// projectIndicator maps an indicator file to a project type
type projectIndicator struct {
	file        string
	projectType ProjectType
	confidence  string
}

// detectionOrder defines the priority order for project detection
// Earlier entries take precedence over later ones
var detectionOrder = []projectIndicator{
	// Go - highest priority
	{"go.mod", ProjectGo, "high"},

	// Rust
	{"Cargo.toml", ProjectRust, "high"},

	// Python (multiple indicators, check in order)
	{"pyproject.toml", ProjectPython, "high"},
	{"setup.py", ProjectPython, "high"},
	{"requirements.txt", ProjectPython, "medium"},
	{"Pipfile", ProjectPython, "high"},

	// Node.js
	{"package.json", ProjectNode, "high"},

	// Java
	{"pom.xml", ProjectJava, "high"},
	{"build.gradle", ProjectJava, "high"},
	{"build.gradle.kts", ProjectJava, "high"},

	// Ruby
	{"Gemfile", ProjectRuby, "high"},

	// PHP
	{"composer.json", ProjectPHP, "high"},

	// Elixir
	{"mix.exs", ProjectElixir, "high"},

	// Swift
	{"Package.swift", ProjectSwift, "high"},

	// C/C++ - only CMake and Meson, NOT standalone Makefile
	{"CMakeLists.txt", ProjectCpp, "high"},
	{"meson.build", ProjectCpp, "high"},
}

// DetectProject detects the project type and package manager in the given directory
func DetectProject(dir string) *DetectionResult {
	result := &DetectionResult{
		ProjectType:    ProjectGeneric,
		PackageManager: "",
		Confidence:     "low",
		IndicatorFile:  "",
	}

	// Check each indicator in priority order
	for _, indicator := range detectionOrder {
		indicatorPath := filepath.Join(dir, indicator.file)
		if fileExists(indicatorPath) {
			result.ProjectType = indicator.projectType
			result.Confidence = indicator.confidence
			result.IndicatorFile = indicator.file
			break
		}
	}

	// Detect package manager based on project type
	result.PackageManager = detectPackageManager(dir, result.ProjectType)

	return result
}

// detectPackageManager detects the package manager for a given project type
func detectPackageManager(dir string, projectType ProjectType) PackageManager {
	switch projectType {
	case ProjectNode:
		return detectNodePackageManager(dir)
	case ProjectPython:
		return detectPythonPackageManager(dir)
	case ProjectJava:
		return detectJavaPackageManager(dir)
	case ProjectCpp:
		return detectCppBuildSystem(dir)
	default:
		return ""
	}
}

// detectNodePackageManager detects npm, yarn, or pnpm
func detectNodePackageManager(dir string) PackageManager {
	// Check lockfiles in order of specificity
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return PMPnpm
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return PMYarn
	}
	// Default to npm
	return PMNpm
}

// detectPythonPackageManager detects pip, poetry, pipenv, or uv
func detectPythonPackageManager(dir string) PackageManager {
	// Check lockfiles and config files in order of specificity
	if fileExists(filepath.Join(dir, "poetry.lock")) {
		return PMPoetry
	}
	if fileExists(filepath.Join(dir, "Pipfile.lock")) || fileExists(filepath.Join(dir, "Pipfile")) {
		return PMPipenv
	}
	if fileExists(filepath.Join(dir, "uv.lock")) {
		return PMUv
	}
	// Default to pip
	return PMPip
}

// detectJavaPackageManager detects maven or gradle
func detectJavaPackageManager(dir string) PackageManager {
	if fileExists(filepath.Join(dir, "pom.xml")) {
		return PMMaven
	}
	if fileExists(filepath.Join(dir, "build.gradle")) || fileExists(filepath.Join(dir, "build.gradle.kts")) {
		return PMGradle
	}
	// Default to maven
	return PMMaven
}

// detectCppBuildSystem detects cmake, meson, or make
func detectCppBuildSystem(dir string) PackageManager {
	if fileExists(filepath.Join(dir, "CMakeLists.txt")) {
		return PMCMake
	}
	if fileExists(filepath.Join(dir, "meson.build")) {
		return PMMeson
	}
	// Default to make (though we won't reach here since Makefile alone doesn't trigger C++)
	return PMMake
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
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
