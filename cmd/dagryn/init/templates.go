package initcmd

import "fmt"

// GetTemplate returns the dagryn.toml template for a given project type and package manager
func GetTemplate(projectType ProjectType, pm PackageManager) string {
	switch projectType {
	case ProjectGo:
		return formatHeader("Go Project", "# This configuration defines a CI workflow for Go projects.\n# Customize the tasks below to match your project structure.") + templateGo
	case ProjectRust:
		return formatHeader("Rust Project", "# This configuration defines a CI workflow for Rust projects using Cargo.\n# Customize the tasks below to match your project structure.") + templateRust
	case ProjectPython:
		return getPythonTemplate(pm)
	case ProjectNode:
		return getNodeTemplate(pm)
	case ProjectJava:
		return getJavaTemplate(pm)
	case ProjectRuby:
		return formatHeader("Ruby Project", "# This configuration defines a CI workflow for Ruby projects using Bundler.\n# Ruby tools like rubocop are typically managed via Gemfile and run via\n# \"bundle exec\". The [plugins] section can be used for tools not in your Gemfile.") + templateRuby
	case ProjectPHP:
		return formatHeader("PHP Project", "# This configuration defines a CI workflow for PHP projects using Composer.\n# Customize the tasks below to match your project structure.") + templatePHP
	case ProjectElixir:
		return formatHeader("Elixir Project", "# This configuration defines a CI workflow for Elixir projects using Mix.\n# Customize the tasks below to match your project structure.") + templateElixir
	case ProjectSwift:
		return formatHeader("Swift Project", "# This configuration defines a CI workflow for Swift projects using SPM.\n# Customize the tasks below to match your project structure.") + templateSwift
	case ProjectCpp:
		return getCppTemplate(pm)
	default:
		return formatHeader("Generic Project", "# Dagryn couldn't auto-detect your project type.\n# Customize the tasks below to match your build system.") + templateGeneric
	}
}

// GetTemplateInfo returns template name and description for listing
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

// getPythonTemplate returns the appropriate Python template based on package manager
func getPythonTemplate(pm PackageManager) string {
	switch pm {
	case PMPoetry:
		return formatHeader("Python Project (Poetry)", "# This configuration defines a CI workflow for Python projects using Poetry.\n# Poetry manages project dependencies, so tools like ruff/black are typically\n# included in pyproject.toml dev-dependencies and run via \"poetry run\".\n# Alternatively, use the [plugins] section to auto-install tools independently.") + templatePythonPoetry
	case PMPipenv:
		return formatHeader("Python Project (Pipenv)", "# This configuration defines a CI workflow for Python projects using Pipenv.\n# Pipenv manages project dependencies, so tools like ruff/black are typically\n# included in Pipfile dev-dependencies and run via \"pipenv run\".") + templatePythonPipenv
	case PMUv:
		return formatHeader("Python Project (uv)", "# This configuration defines a CI workflow for Python projects using uv.\n# uv is a fast Python package installer and resolver.\n# uv manages project dependencies, so tools like ruff are typically\n# included in pyproject.toml and run via \"uv run\".") + templatePythonUv
	default:
		return formatHeader("Python Project (pip)", "# This configuration defines a CI workflow for Python projects using pip.\n# Customize the tasks below to match your project structure.") + templatePythonPip
	}
}

// getNodeTemplate returns the appropriate Node.js template based on package manager
func getNodeTemplate(pm PackageManager) string {
	switch pm {
	case PMYarn:
		return formatHeader("Node.js Project (Yarn)", "# This configuration defines a CI workflow for Node.js projects using Yarn.\n# Node.js tools like eslint/prettier are typically managed via package.json\n# and run via yarn scripts. The [plugins] section can be used for tools\n# not in your package.json (e.g., global CLI tools).") + templateNodeYarn
	case PMPnpm:
		return formatHeader("Node.js Project (pnpm)", "# This configuration defines a CI workflow for Node.js projects using pnpm.\n# Node.js tools like eslint/prettier are typically managed via package.json\n# and run via pnpm scripts. The [plugins] section can be used for tools\n# not in your package.json (e.g., global CLI tools).") + templateNodePnpm
	default:
		return formatHeader("Node.js Project (npm)", "# This configuration defines a CI workflow for Node.js projects using npm.\n# Node.js tools like eslint/prettier are typically managed via package.json\n# and run via npm scripts. The [plugins] section can be used for tools\n# not in your package.json (e.g., global CLI tools).") + templateNodeNpm
	}
}

// getJavaTemplate returns the appropriate Java template based on build tool
func getJavaTemplate(pm PackageManager) string {
	switch pm {
	case PMGradle:
		return formatHeader("Java Project (Gradle)", "# This configuration defines a CI workflow for Java projects using Gradle.\n# Customize the tasks below to match your project structure.") + templateJavaGradle
	default:
		return formatHeader("Java Project (Maven)", "# This configuration defines a CI workflow for Java projects using Maven.\n# Customize the tasks below to match your project structure.") + templateJavaMaven
	}
}

// getCppTemplate returns the appropriate C/C++ template based on build system
func getCppTemplate(pm PackageManager) string {
	switch pm {
	case PMMeson:
		return formatHeader("C/C++ Project (Meson)", "# This configuration defines a CI workflow for C/C++ projects using Meson.\n# Customize the tasks below to match your project structure.") + templateCppMeson
	default:
		return formatHeader("C/C++ Project (CMake)", "# This configuration defines a CI workflow for C/C++ projects using CMake.\n# Customize the tasks below to match your project structure.") + templateCppCMake
	}
}

// formatHeader generates a consistent header for templates
func formatHeader(projectType, description string) string {
	return fmt.Sprintf(`# dagryn.toml - %s
# Generated by: dagryn init
#
%s
#
# Workflow configuration:
#   [workflow]
#   name = "ci"                       # Workflow name
#   default = true                    # Set as the default workflow
#
#   [workflow.trigger]                # Optional: filter which events trigger this workflow
#   [workflow.trigger.push]
#   branches = ["main", "develop"]    # Only trigger on pushes to these branches
#   [workflow.trigger.pull_request]
#   branches = ["main"]               # Only trigger on PRs targeting these branches
#   types = ["opened", "synchronize"] # PR event types to match
#
# Available fields for each task:
#   command   - The shell command to run (required)
#   needs     - List of tasks that must complete first
#   inputs    - File globs that affect task caching
#   outputs   - File globs produced by this task
#   timeout   - Maximum execution time (e.g., "5m", "1h")
#   workdir   - Working directory for the command
#   env       - Environment variables as key=value pairs
#   uses      - Plugin(s) to install before running the task
#   group     - Logical group name for organizing related tasks
#   if        - Condition expression to control whether the task runs
#
# Cache configuration:
#   [cache]
#   enabled = true                    # Enable/disable local caching (default: true)
#   dir = ""             # Override local cache directory
#
#   [cache.remote]
#   enabled = true                    # Enable remote cache sharing
#   cloud = true                      # Use Dagryn Cloud cache (requires "dagryn login")
#   strategy = "local-first"          # "local-first", "remote-first", or "write-through"
#   fallback_on_error = true          # Fall back to local cache on remote errors
#
#   # Self-hosted remote cache (when cloud = false):
#   provider = "s3"                   # "s3" or "filesystem"
#   bucket = "my-cache-bucket"
#   region = "us-east-1"
#   endpoint = ""                     # Custom S3 endpoint (for R2, MinIO, etc.)
#   access_key_id = ""
#   secret_access_key = ""
#   prefix = ""                       # Key prefix in the bucket
#
# AI analysis configuration (optional, enables AI-powered failure analysis):
#   [ai]
#   enabled = true                    # Enable AI analysis (default: false)
#   mode = "summarize"                # "summarize" or "summarize_and_suggest"
#   provider = "openai"               # "openai", "google", or "gemini"
#   model = "gpt-4o"                  # Model name (default varies by provider)
#
#   [ai.backend]
#   mode = "byok"                     # "byok" (bring your own key), "managed", or "agent"
#
#   [ai.backend.byok]
#   api_key_env = "OPENAI_API_KEY"    # Env var containing the API key
#
#   [ai.guardrails]
#   min_confidence = 0.7              # Minimum confidence threshold (0.0-1.0)
#   max_suggestions_per_analysis = 5  # Max suggestions per run
#
#   [ai.publish]
#   github_comment = true             # Post analysis as PR comment
#   github_check = true               # Create GitHub check run
#   github_suggestions = false        # Post inline code suggestions

`, projectType, description)
}

// =============================================================================
// Go Template
// =============================================================================

var templateGo = `[workflow]
name = "ci"
default = true

# Global plugins (available to all tasks by reference name)
[plugins]
golangci-lint = "github:golangci/golangci-lint@v2.8.0"

# Build the Go application
[tasks.build]
command = "go build -v ./..."
inputs = ["**/*.go", "go.mod", "go.sum"]
outputs = ["bin/**"]
timeout = "10m"

# Run tests with coverage
[tasks.test]
command = "go test -v -race -coverprofile=coverage.out ./..."
needs = ["build"]
inputs = ["**/*.go", "go.mod", "go.sum"]
outputs = ["coverage.out"]
timeout = "5m"

# Run linter (auto-installed via plugin)
[tasks.lint]
uses = ["golangci-lint"]
command = "golangci-lint run ./..."
inputs = ["**/*.go", ".golangci.yml", ".golangci.yaml"]
timeout = "2m"

# Check code formatting (fails if files need formatting)
[tasks.fmt]
command = "test -z \"$(gofmt -l .)\""
inputs = ["**/*.go"]
timeout = "1m"
`

// =============================================================================
// Rust Template
// =============================================================================

var templateRust = `[workflow]
name = "ci"
default = true

# Build the project in release mode
[tasks.build]
command = "cargo build --release"
inputs = ["src/**/*.rs", "Cargo.toml", "Cargo.lock"]
outputs = ["target/release/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "cargo test"
needs = ["build"]
inputs = ["src/**/*.rs", "tests/**/*.rs", "Cargo.toml", "Cargo.lock"]
timeout = "5m"

# Run clippy linter
[tasks.lint]
command = "cargo clippy -- -D warnings"
inputs = ["src/**/*.rs", "Cargo.toml"]
timeout = "2m"

# Check code formatting
[tasks.fmt]
command = "cargo fmt -- --check"
inputs = ["src/**/*.rs"]
timeout = "1m"
`

// =============================================================================
// Python Templates
// =============================================================================

var templatePythonPip = `[workflow]
name = "ci"
default = true

# Global plugins (available to all tasks by reference name)
[plugins]
ruff = "pip:ruff@0.8.0"
black = "pip:black@24.10.0"
pytest = "pip:pytest@8.3.0"
pytest-cov = "pip:pytest-cov@6.0.0"

# Install dependencies
[tasks.install]
command = "pip install -r requirements.txt"
inputs = ["requirements.txt", "requirements-dev.txt"]
timeout = "5m"

# Run tests with pytest (auto-installed via plugin)
[tasks.test]
uses = ["pytest", "pytest-cov"]
command = "pytest -v --cov=src"
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml", "pytest.ini"]
outputs = [".coverage", "htmlcov/**"]
timeout = "5m"

# Run linter (auto-installed via plugin)
[tasks.lint]
uses = ["ruff"]
command = "ruff check ."
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml", "ruff.toml"]
timeout = "2m"

# Check code formatting (auto-installed via plugin)
[tasks.fmt]
uses = ["black"]
command = "black --check ."
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml"]
timeout = "2m"
`

var templatePythonPoetry = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "poetry install"
inputs = ["pyproject.toml", "poetry.lock"]
timeout = "5m"

# Run tests with pytest
[tasks.test]
command = "poetry run pytest -v --cov=src"
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml"]
outputs = [".coverage", "htmlcov/**"]
timeout = "5m"

# Run linter
[tasks.lint]
command = "poetry run ruff check ."
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml", "ruff.toml"]
timeout = "2m"

# Check code formatting
[tasks.fmt]
command = "poetry run black --check ."
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml"]
timeout = "2m"
`

var templatePythonPipenv = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "pipenv install --dev"
inputs = ["Pipfile", "Pipfile.lock"]
timeout = "5m"

# Run tests with pytest
[tasks.test]
command = "pipenv run pytest -v --cov=src"
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "Pipfile"]
outputs = [".coverage", "htmlcov/**"]
timeout = "5m"

# Run linter
[tasks.lint]
command = "pipenv run ruff check ."
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "Pipfile", "ruff.toml"]
timeout = "2m"

# Check code formatting
[tasks.fmt]
command = "pipenv run black --check ."
needs = ["install"]
inputs = ["src/**/*.py", "tests/**/*.py", "Pipfile"]
timeout = "2m"
`

var templatePythonUv = `[workflow]
name = "ci"
default = true

# Sync dependencies
[tasks.sync]
command = "uv sync"
inputs = ["pyproject.toml", "uv.lock"]
timeout = "5m"

# Run tests with pytest
[tasks.test]
command = "uv run pytest -v --cov=src"
needs = ["sync"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml"]
outputs = [".coverage", "htmlcov/**"]
timeout = "5m"

# Run linter
[tasks.lint]
command = "uv run ruff check ."
needs = ["sync"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml", "ruff.toml"]
timeout = "2m"

# Check code formatting
[tasks.fmt]
command = "uv run ruff format --check ."
needs = ["sync"]
inputs = ["src/**/*.py", "tests/**/*.py", "pyproject.toml"]
timeout = "2m"
`

// =============================================================================
// Node.js Templates
// =============================================================================

var templateNodeNpm = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "npm ci"
inputs = ["package.json", "package-lock.json"]
outputs = ["node_modules/**"]
timeout = "5m"

# Build the project
[tasks.build]
command = "npm run build"
needs = ["install"]
inputs = ["src/**", "package.json", "tsconfig.json"]
outputs = ["dist/**", "build/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "npm test"
needs = ["build"]
inputs = ["src/**", "test/**", "tests/**", "__tests__/**", "package.json"]
timeout = "5m"

# Run linter (configured in package.json scripts)
[tasks.lint]
command = "npm run lint"
needs = ["install"]
inputs = ["src/**", ".eslintrc*", "eslint.config.*", "package.json"]
timeout = "2m"
`

var templateNodeYarn = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "yarn install --frozen-lockfile"
inputs = ["package.json", "yarn.lock"]
outputs = ["node_modules/**"]
timeout = "5m"

# Build the project
[tasks.build]
command = "yarn build"
needs = ["install"]
inputs = ["src/**", "package.json", "tsconfig.json"]
outputs = ["dist/**", "build/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "yarn test"
needs = ["build"]
inputs = ["src/**", "test/**", "tests/**", "__tests__/**", "package.json"]
timeout = "5m"

# Run linter (configured in package.json scripts)
[tasks.lint]
command = "yarn lint"
needs = ["install"]
inputs = ["src/**", ".eslintrc*", "eslint.config.*", "package.json"]
timeout = "2m"
`

var templateNodePnpm = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "pnpm install --frozen-lockfile"
inputs = ["package.json", "pnpm-lock.yaml"]
outputs = ["node_modules/**"]
timeout = "5m"

# Build the project
[tasks.build]
command = "pnpm build"
needs = ["install"]
inputs = ["src/**", "package.json", "tsconfig.json"]
outputs = ["dist/**", "build/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "pnpm test"
needs = ["build"]
inputs = ["src/**", "test/**", "tests/**", "__tests__/**", "package.json"]
timeout = "5m"

# Run linter (configured in package.json scripts)
[tasks.lint]
command = "pnpm lint"
needs = ["install"]
inputs = ["src/**", ".eslintrc*", "eslint.config.*", "package.json"]
timeout = "2m"
`

// =============================================================================
// Java Templates
// =============================================================================

var templateJavaMaven = `[workflow]
name = "ci"
default = true

# Compile the project
[tasks.compile]
command = "mvn compile -q"
inputs = ["src/main/**/*.java", "pom.xml"]
outputs = ["target/classes/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "mvn test -q"
needs = ["compile"]
inputs = ["src/**/*.java", "pom.xml"]
outputs = ["target/surefire-reports/**"]
timeout = "5m"

# Package the application
[tasks.package]
command = "mvn package -q -DskipTests"
needs = ["test"]
inputs = ["src/**/*.java", "pom.xml"]
outputs = ["target/*.jar", "target/*.war"]
timeout = "10m"

# Run static analysis with SpotBugs (optional, requires plugin)
[tasks.lint]
command = "mvn spotbugs:check -q || true"
needs = ["compile"]
inputs = ["src/main/**/*.java", "pom.xml"]
timeout = "2m"
`

var templateJavaGradle = `[workflow]
name = "ci"
default = true

# Build the project
[tasks.build]
command = "./gradlew build -x test"
inputs = ["src/main/**/*.java", "build.gradle*", "settings.gradle*", "gradle/**"]
outputs = ["build/classes/**", "build/libs/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "./gradlew test"
needs = ["build"]
inputs = ["src/**/*.java", "build.gradle*"]
outputs = ["build/reports/tests/**"]
timeout = "5m"

# Run checks (includes static analysis if configured)
[tasks.check]
command = "./gradlew check -x test"
needs = ["build"]
inputs = ["src/**/*.java", "build.gradle*"]
timeout = "2m"

# Clean build artifacts
[tasks.clean]
command = "./gradlew clean"
timeout = "1m"
`

// =============================================================================
// Ruby Template
// =============================================================================

var templateRuby = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "bundle install"
inputs = ["Gemfile", "Gemfile.lock"]
outputs = ["vendor/bundle/**"]
timeout = "5m"

# Run tests with RSpec or Minitest
[tasks.test]
command = "bundle exec rake test || bundle exec rspec"
needs = ["install"]
inputs = ["lib/**/*.rb", "app/**/*.rb", "spec/**/*.rb", "test/**/*.rb", "Gemfile"]
timeout = "5m"

# Run RuboCop linter (configured in Gemfile)
[tasks.lint]
command = "bundle exec rubocop"
needs = ["install"]
inputs = ["lib/**/*.rb", "app/**/*.rb", "spec/**/*.rb", ".rubocop.yml"]
timeout = "2m"
`

// =============================================================================
// PHP Template
// =============================================================================

var templatePHP = `[workflow]
name = "ci"
default = true

# Install dependencies
[tasks.install]
command = "composer install --no-interaction --prefer-dist"
inputs = ["composer.json", "composer.lock"]
outputs = ["vendor/**"]
timeout = "5m"

# Run tests with PHPUnit
[tasks.test]
command = "vendor/bin/phpunit"
needs = ["install"]
inputs = ["src/**/*.php", "tests/**/*.php", "phpunit.xml*"]
outputs = ["coverage/**"]
timeout = "5m"

# Run static analysis with PHPStan
[tasks.lint]
command = "vendor/bin/phpstan analyse || true"
needs = ["install"]
inputs = ["src/**/*.php", "phpstan.neon*"]
timeout = "2m"

# Check code style with PHP-CS-Fixer
[tasks.fmt]
command = "vendor/bin/php-cs-fixer fix --dry-run --diff"
needs = ["install"]
inputs = ["src/**/*.php", ".php-cs-fixer.php"]
timeout = "2m"
`

// =============================================================================
// Elixir Template
// =============================================================================

var templateElixir = `[workflow]
name = "ci"
default = true

# Get dependencies
[tasks.deps]
command = "mix deps.get"
inputs = ["mix.exs", "mix.lock"]
outputs = ["deps/**"]
timeout = "5m"

# Compile the project
[tasks.compile]
command = "mix compile --warnings-as-errors"
needs = ["deps"]
inputs = ["lib/**/*.ex", "mix.exs"]
outputs = ["_build/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "mix test"
needs = ["compile"]
inputs = ["lib/**/*.ex", "test/**/*.exs", "mix.exs"]
timeout = "5m"

# Check code formatting
[tasks.format]
command = "mix format --check-formatted"
needs = ["deps"]
inputs = ["lib/**/*.ex", "test/**/*.exs", ".formatter.exs"]
timeout = "1m"

# Run Credo for static analysis
[tasks.lint]
command = "mix credo --strict || true"
needs = ["compile"]
inputs = ["lib/**/*.ex", ".credo.exs"]
timeout = "2m"
`

// =============================================================================
// Swift Template
// =============================================================================

var templateSwift = `[workflow]
name = "ci"
default = true

# Global plugins (available to all tasks by reference name)
[plugins]
swift-format = "github:apple/swift-format@510.1.0"

# Build the project
[tasks.build]
command = "swift build"
inputs = ["Sources/**/*.swift", "Package.swift", "Package.resolved"]
outputs = [".build/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "swift test"
needs = ["build"]
inputs = ["Sources/**/*.swift", "Tests/**/*.swift", "Package.swift"]
timeout = "5m"

# Check code formatting (auto-installed via plugin)
[tasks.fmt]
uses = ["swift-format"]
command = "swift-format lint --recursive Sources Tests"
inputs = ["Sources/**/*.swift", "Tests/**/*.swift", ".swift-format"]
timeout = "2m"
`

// =============================================================================
// C/C++ Templates
// =============================================================================

var templateCppCMake = `[workflow]
name = "ci"
default = true

# Configure the build
[tasks.configure]
command = "cmake -B build -DCMAKE_BUILD_TYPE=Release"
inputs = ["CMakeLists.txt", "cmake/**"]
outputs = ["build/CMakeCache.txt"]
timeout = "2m"

# Build the project
[tasks.build]
command = "cmake --build build --parallel"
needs = ["configure"]
inputs = ["src/**/*.cpp", "src/**/*.c", "src/**/*.h", "include/**/*.h", "CMakeLists.txt"]
outputs = ["build/**"]
timeout = "10m"

# Run tests with CTest
[tasks.test]
command = "ctest --test-dir build --output-on-failure"
needs = ["build"]
inputs = ["src/**/*.cpp", "tests/**/*.cpp", "CMakeLists.txt"]
timeout = "5m"

# Clean build artifacts
[tasks.clean]
command = "rm -rf build"
timeout = "1m"
`

var templateCppMeson = `[workflow]
name = "ci"
default = true

# Configure the build
[tasks.configure]
command = "meson setup build --buildtype=release"
inputs = ["meson.build", "meson_options.txt"]
outputs = ["build/build.ninja"]
timeout = "2m"

# Build the project
[tasks.build]
command = "meson compile -C build"
needs = ["configure"]
inputs = ["src/**/*.cpp", "src/**/*.c", "src/**/*.h", "include/**/*.h", "meson.build"]
outputs = ["build/**"]
timeout = "10m"

# Run tests
[tasks.test]
command = "meson test -C build"
needs = ["build"]
inputs = ["src/**/*.cpp", "tests/**/*.cpp", "meson.build"]
timeout = "5m"

# Clean build artifacts
[tasks.clean]
command = "rm -rf build"
timeout = "1m"
`

// =============================================================================
// Generic Template
// =============================================================================

var templateGeneric = `# Plugin formats:
#   github:owner/repo@version  - Download from GitHub releases
#   go:module/path@version     - Install via go install
#   npm:package@version        - Install via npm
#   pip:package@version        - Install via pip
#   cargo:crate@version        - Install via cargo
#
# Example workflow:
#   install -> build -> test
#                   \-> lint

[workflow]
name = "ci"
default = true

# Uncomment to define global plugins available to all tasks
# [plugins]
# my-tool = "github:owner/my-tool@v1.0.0"

# Install dependencies
# TODO: Replace with your dependency installation command
[tasks.install]
command = "echo 'TODO: Add your install command (e.g., npm install, pip install -r requirements.txt)'"
timeout = "5m"

# Build the project
# TODO: Replace with your build command
[tasks.build]
command = "echo 'TODO: Add your build command (e.g., make, go build, npm run build)'"
needs = ["install"]
timeout = "10m"

# Run tests
# TODO: Replace with your test command
[tasks.test]
command = "echo 'TODO: Add your test command (e.g., make test, go test, npm test)'"
needs = ["build"]
timeout = "5m"

# Run linter
# TODO: Replace with your lint command
# Example with plugin: uses = ["my-tool"]
[tasks.lint]
command = "echo 'TODO: Add your lint command (e.g., golangci-lint run, npm run lint)'"
needs = ["install"]
timeout = "2m"
`
