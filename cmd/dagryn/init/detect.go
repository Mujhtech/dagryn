package initcmd

import (
	"os"
	"path/filepath"

	"github.com/mujhtech/dagryn/pkg/templates"
)

// Type aliases so existing code in this package and other callers continues to work.
type ProjectType = templates.ProjectType
type PackageManager = templates.PackageManager

// Re-export project type constants.
const (
	ProjectGo      = templates.ProjectGo
	ProjectRust    = templates.ProjectRust
	ProjectPython  = templates.ProjectPython
	ProjectNode    = templates.ProjectNode
	ProjectJava    = templates.ProjectJava
	ProjectRuby    = templates.ProjectRuby
	ProjectPHP     = templates.ProjectPHP
	ProjectElixir  = templates.ProjectElixir
	ProjectSwift   = templates.ProjectSwift
	ProjectCpp     = templates.ProjectCpp
	ProjectGeneric = templates.ProjectGeneric
)

// Re-export package manager constants.
const (
	PMNpm    = templates.PMNpm
	PMYarn   = templates.PMYarn
	PMPnpm   = templates.PMPnpm
	PMPip    = templates.PMPip
	PMPoetry = templates.PMPoetry
	PMPipenv = templates.PMPipenv
	PMUv     = templates.PMUv
	PMMaven  = templates.PMMaven
	PMGradle = templates.PMGradle
	PMCMake  = templates.PMCMake
	PMMeson  = templates.PMMeson
	PMMake   = templates.PMMake
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

// AllProjectTypes delegates to the shared package.
func AllProjectTypes() []ProjectType {
	return templates.AllProjectTypes()
}

// ProjectTypeFromString delegates to the shared package.
func ProjectTypeFromString(s string) ProjectType {
	return templates.ProjectTypeFromString(s)
}
