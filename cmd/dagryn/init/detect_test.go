package initcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createFile creates a file with the given name in the directory
func createFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)
}

func TestDetectProject_Go(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "go.mod")

	result := DetectProject(dir)

	assert.Equal(t, ProjectGo, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
	assert.Equal(t, "go.mod", result.IndicatorFile)
}

func TestDetectProject_Rust(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "Cargo.toml")

	result := DetectProject(dir)

	assert.Equal(t, ProjectRust, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
	assert.Equal(t, "Cargo.toml", result.IndicatorFile)
}

func TestDetectProject_Python_Pyproject(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "pyproject.toml")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
	assert.Equal(t, PMPip, result.PackageManager) // default
	assert.Equal(t, "high", result.Confidence)
	assert.Equal(t, "pyproject.toml", result.IndicatorFile)
}

func TestDetectProject_Python_SetupPy(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "setup.py")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
	assert.Equal(t, PMPip, result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Python_Requirements(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "requirements.txt")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
	assert.Equal(t, PMPip, result.PackageManager)
	assert.Equal(t, "medium", result.Confidence) // lower confidence for requirements.txt
}

func TestDetectProject_Python_Poetry(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "pyproject.toml")
	createFile(t, dir, "poetry.lock")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
	assert.Equal(t, PMPoetry, result.PackageManager)
}

func TestDetectProject_Python_Pipenv(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "Pipfile")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
	assert.Equal(t, PMPipenv, result.PackageManager)
}

func TestDetectProject_Python_Uv(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "pyproject.toml")
	createFile(t, dir, "uv.lock")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
	assert.Equal(t, PMUv, result.PackageManager)
}

func TestDetectProject_Node_Npm(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "package.json")

	result := DetectProject(dir)

	assert.Equal(t, ProjectNode, result.ProjectType)
	assert.Equal(t, PMNpm, result.PackageManager) // default
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Node_Yarn(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "package.json")
	createFile(t, dir, "yarn.lock")

	result := DetectProject(dir)

	assert.Equal(t, ProjectNode, result.ProjectType)
	assert.Equal(t, PMYarn, result.PackageManager)
}

func TestDetectProject_Node_Pnpm(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "package.json")
	createFile(t, dir, "pnpm-lock.yaml")

	result := DetectProject(dir)

	assert.Equal(t, ProjectNode, result.ProjectType)
	assert.Equal(t, PMPnpm, result.PackageManager)
}

func TestDetectProject_Java_Maven(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "pom.xml")

	result := DetectProject(dir)

	assert.Equal(t, ProjectJava, result.ProjectType)
	assert.Equal(t, PMMaven, result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Java_Gradle(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "build.gradle")

	result := DetectProject(dir)

	assert.Equal(t, ProjectJava, result.ProjectType)
	assert.Equal(t, PMGradle, result.PackageManager)
}

func TestDetectProject_Java_GradleKts(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "build.gradle.kts")

	result := DetectProject(dir)

	assert.Equal(t, ProjectJava, result.ProjectType)
	assert.Equal(t, PMGradle, result.PackageManager)
}

func TestDetectProject_Ruby(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "Gemfile")

	result := DetectProject(dir)

	assert.Equal(t, ProjectRuby, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_PHP(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "composer.json")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPHP, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Elixir(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "mix.exs")

	result := DetectProject(dir)

	assert.Equal(t, ProjectElixir, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Swift(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "Package.swift")

	result := DetectProject(dir)

	assert.Equal(t, ProjectSwift, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Cpp_CMake(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "CMakeLists.txt")

	result := DetectProject(dir)

	assert.Equal(t, ProjectCpp, result.ProjectType)
	assert.Equal(t, PMCMake, result.PackageManager)
	assert.Equal(t, "high", result.Confidence)
}

func TestDetectProject_Cpp_Meson(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "meson.build")

	result := DetectProject(dir)

	assert.Equal(t, ProjectCpp, result.ProjectType)
	assert.Equal(t, PMMeson, result.PackageManager)
}

func TestDetectProject_Makefile_IsGeneric(t *testing.T) {
	// Standalone Makefile should NOT trigger C/C++ detection
	dir := t.TempDir()
	createFile(t, dir, "Makefile")

	result := DetectProject(dir)

	assert.Equal(t, ProjectGeneric, result.ProjectType)
	assert.Equal(t, "low", result.Confidence)
}

func TestDetectProject_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result := DetectProject(dir)

	assert.Equal(t, ProjectGeneric, result.ProjectType)
	assert.Equal(t, PackageManager(""), result.PackageManager)
	assert.Equal(t, "low", result.Confidence)
	assert.Equal(t, "", result.IndicatorFile)
}

func TestDetectProject_Priority_GoOverNode(t *testing.T) {
	// Go should take priority over Node.js in a multi-language project
	dir := t.TempDir()
	createFile(t, dir, "go.mod")
	createFile(t, dir, "package.json")

	result := DetectProject(dir)

	assert.Equal(t, ProjectGo, result.ProjectType)
	assert.Equal(t, "go.mod", result.IndicatorFile)
}

func TestDetectProject_Priority_RustOverPython(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "Cargo.toml")
	createFile(t, dir, "requirements.txt")

	result := DetectProject(dir)

	assert.Equal(t, ProjectRust, result.ProjectType)
}

func TestDetectProject_Priority_PythonOverNode(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "pyproject.toml")
	createFile(t, dir, "package.json")

	result := DetectProject(dir)

	assert.Equal(t, ProjectPython, result.ProjectType)
}

func TestProjectTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected ProjectType
	}{
		{"go", ProjectGo},
		{"rust", ProjectRust},
		{"python", ProjectPython},
		{"node", ProjectNode},
		{"nodejs", ProjectNode},
		{"js", ProjectNode},
		{"javascript", ProjectNode},
		{"java", ProjectJava},
		{"ruby", ProjectRuby},
		{"php", ProjectPHP},
		{"elixir", ProjectElixir},
		{"swift", ProjectSwift},
		{"cpp", ProjectCpp},
		{"c++", ProjectCpp},
		{"c", ProjectCpp},
		{"generic", ProjectGeneric},
		{"unknown", ProjectType("")},
		{"", ProjectType("")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ProjectTypeFromString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProjectType_DisplayName(t *testing.T) {
	tests := []struct {
		pt       ProjectType
		expected string
	}{
		{ProjectGo, "Go"},
		{ProjectRust, "Rust"},
		{ProjectPython, "Python"},
		{ProjectNode, "Node.js"},
		{ProjectJava, "Java"},
		{ProjectRuby, "Ruby"},
		{ProjectPHP, "PHP"},
		{ProjectElixir, "Elixir"},
		{ProjectSwift, "Swift"},
		{ProjectCpp, "C/C++"},
		{ProjectGeneric, "Generic"},
	}

	for _, tt := range tests {
		t.Run(string(tt.pt), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pt.DisplayName())
		})
	}
}

func TestProjectType_Description(t *testing.T) {
	// Just verify descriptions are not empty
	for _, pt := range AllProjectTypes() {
		t.Run(string(pt), func(t *testing.T) {
			desc := pt.Description()
			assert.NotEmpty(t, desc, "Description for %s should not be empty", pt)
		})
	}
}

func TestAllProjectTypes(t *testing.T) {
	types := AllProjectTypes()

	// Should return all 11 types
	assert.Len(t, types, 11)

	// Should contain specific types
	assert.Contains(t, types, ProjectGo)
	assert.Contains(t, types, ProjectRust)
	assert.Contains(t, types, ProjectPython)
	assert.Contains(t, types, ProjectNode)
	assert.Contains(t, types, ProjectJava)
	assert.Contains(t, types, ProjectRuby)
	assert.Contains(t, types, ProjectPHP)
	assert.Contains(t, types, ProjectElixir)
	assert.Contains(t, types, ProjectSwift)
	assert.Contains(t, types, ProjectCpp)
	assert.Contains(t, types, ProjectGeneric)
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	// Non-existent file
	assert.False(t, fileExists(filepath.Join(dir, "nonexistent")))

	// Create a file
	createFile(t, dir, "exists.txt")
	assert.True(t, fileExists(filepath.Join(dir, "exists.txt")))

	// Directory should return false (we want files only)
	subdir := filepath.Join(dir, "subdir")
	err := os.Mkdir(subdir, 0755)
	require.NoError(t, err)
	assert.False(t, fileExists(subdir))
}
