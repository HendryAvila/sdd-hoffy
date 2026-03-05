// Package tools — see helpers.go for package doc.
//
// reverse_engineer.go implements the sdd_reverse_engineer scanner tool.
// It scans an existing project's filesystem and produces a structured
// markdown report for the AI to analyze. The tool is read-only — it
// never writes files. Sub-scanners are functions (not interfaces — YAGNI)
// that each collect one category of evidence.
//
// ADR-003: Sub-scanner functions over monolithic scan.
// ADR-004: Markdown over XML for scan report.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/HendryAvila/Hoofy/internal/memory"
	"github.com/mark3labs/mcp-go/mcp"
)

// --- Scanner types ---

// scanSection holds the result of a single sub-scanner.
type scanSection struct {
	title        string // section heading in the report
	content      string // markdown content
	filesRead    int    // files successfully read
	filesSkipped int    // files detected but skipped (too large, unreadable)
}

// --- Ignored directories ---

// ignoreDirs are directories skipped during tree walks.
// Common build outputs, caches, VCS dirs, and dependency directories.
var ignoreDirs = map[string]bool{
	"node_modules": true, ".git": true, "__pycache__": true,
	"vendor": true, "dist": true, "build": true, "target": true,
	".next": true, ".nuxt": true, "venv": true, ".venv": true,
	".idea": true, ".vscode": true, "coverage": true,
	".cache": true, ".tmp": true, ".terraform": true,
}

// maxFileSize is the maximum file size to read content from (100KB).
const maxFileSize = 100 * 1024

// maxEntryPointLines is the number of lines to read from entry points.
const maxEntryPointLines = 50

// maxConfigLines is the number of lines to read from config files.
const maxConfigLines = 100

// --- Manifest scanner ---

// manifestFiles are package manifest filenames to detect.
var manifestFiles = []string{
	"package.json",
	"go.mod",
	"requirements.txt",
	"pyproject.toml",
	"Cargo.toml",
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
	"Gemfile",
	"composer.json",
	"mix.exs",
	"pubspec.yaml",
}

// scanManifests detects and reads package manifests.
func scanManifests(root, detailLevel string) scanSection {
	s := scanSection{title: "Project Overview"}
	var parts []string
	for _, name := range manifestFiles {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			s.filesSkipped++
			continue
		}
		s.filesRead++

		content := string(data)
		if detailLevel == "summary" {
			// Just report filename and size.
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", name, info.Size()))
		} else {
			parts = append(parts, fmt.Sprintf("### %s\n\n```\n%s\n```", name, truncateContent(content, 3000)))
		}
	}

	if len(parts) == 0 {
		s.content = "_No package manifests found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- Structure scanner ---

// scanStructure builds a directory tree with depth limiting.
func scanStructure(root, detailLevel string, maxDepth int) scanSection {
	s := scanSection{title: "Directory Structure"}
	if maxDepth <= 0 {
		maxDepth = 3
	}

	var lines []string
	lines = append(lines, "```")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // graceful degradation
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			lines = append(lines, filepath.Base(root)+"/")
			return nil
		}

		// Skip ignored directories.
		if d.IsDir() && ignoreDirs[d.Name()] {
			return filepath.SkipDir
		}

		// Depth check.
		depth := strings.Count(rel, string(filepath.Separator))
		if d.IsDir() {
			if depth >= maxDepth {
				return filepath.SkipDir
			}
			indent := strings.Repeat("  ", depth)
			lines = append(lines, fmt.Sprintf("%s%s/", indent, d.Name()))
		} else if detailLevel != "summary" && depth < maxDepth {
			indent := strings.Repeat("  ", depth)
			lines = append(lines, fmt.Sprintf("%s%s", indent, d.Name()))
			s.filesRead++
		}

		return nil
	})

	lines = append(lines, "```")

	if err != nil {
		s.content = fmt.Sprintf("_Error scanning directory: %v_", err)
	} else {
		s.content = strings.Join(lines, "\n")
	}
	return s
}

// --- Config scanner ---

// configFiles are configuration files to detect.
var configFiles = []string{
	"tsconfig.json",
	"tsconfig.base.json",
	".eslintrc",
	".eslintrc.js",
	".eslintrc.json",
	".eslintrc.yml",
	"eslint.config.js",
	"eslint.config.mjs",
	"biome.json",
	"biome.jsonc",
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	".dockerignore",
	"Makefile",
	".env.example",
	".env.sample",
	"Procfile",
	"fly.toml",
	"vercel.json",
	"netlify.toml",
	"render.yaml",
	"railway.json",
	"turbo.json",
	"nx.json",
	"lerna.json",
}

// configDirs are directories to scan for CI config files.
var configDirs = []struct {
	dir     string
	pattern string
}{
	{".github/workflows", "*.yml"},
	{".github/workflows", "*.yaml"},
	{".circleci", "*.yml"},
}

// scanConfigs detects and reads configuration files.
func scanConfigs(root, detailLevel string) scanSection {
	s := scanSection{title: "Tech Stack Evidence"}
	var parts []string

	// Individual config files.
	for _, name := range configFiles {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			parts = append(parts, fmt.Sprintf("- **%s** (skipped: %d bytes > 100KB)", name, info.Size()))
			continue
		}

		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", name, info.Size()))
		} else {
			content := readFirstLines(path, maxConfigLines)
			if content != "" {
				parts = append(parts, fmt.Sprintf("### %s\n\n```\n%s\n```", name, content))
			}
		}
	}

	// CI config directories.
	for _, cd := range configDirs {
		dirPath := filepath.Join(root, cd.dir)
		matches, err := filepath.Glob(filepath.Join(dirPath, cd.pattern))
		if err != nil || len(matches) == 0 {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			relPath, _ := filepath.Rel(root, match)
			if info.Size() > maxFileSize {
				s.filesSkipped++
				continue
			}
			s.filesRead++
			if detailLevel == "summary" {
				parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", relPath, info.Size()))
			} else {
				content := readFirstLines(match, maxConfigLines)
				if content != "" {
					parts = append(parts, fmt.Sprintf("### %s\n\n```yaml\n%s\n```", relPath, content))
				}
			}
		}
	}

	if len(parts) == 0 {
		s.content = "_No configuration files found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- Entry point scanner ---

// entryPointPatterns are file paths to check as entry points.
var entryPointPatterns = []string{
	"main.go",
	"index.ts",
	"index.js",
	"index.tsx",
	"index.jsx",
	"app.py",
	"manage.py",
	"main.py",
	"src/main.rs",
	"src/lib.rs",
	"src/index.ts",
	"src/index.js",
	"src/index.tsx",
	"src/main.ts",
	"src/main.js",
	"src/app.ts",
	"src/app.js",
	"server.ts",
	"server.js",
	"app.ts",
	"app.js",
}

// entryPointGlobs are glob patterns for multi-command Go binaries.
var entryPointGlobs = []string{
	"cmd/*/main.go",
}

// scanEntryPoints detects and reads project entry points.
func scanEntryPoints(root, detailLevel string) scanSection {
	s := scanSection{title: "Architecture Evidence"}
	var parts []string

	// Check exact paths first.
	for _, ep := range entryPointPatterns {
		path := filepath.Join(root, ep)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			continue
		}
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", ep, info.Size()))
		} else {
			content := readFirstLines(path, maxEntryPointLines)
			if content != "" {
				ext := filepath.Ext(ep)
				lang := langFromExt(ext)
				parts = append(parts, fmt.Sprintf("### %s\n\n```%s\n%s\n```", ep, lang, content))
			}
		}
	}

	// Check glob patterns (cmd/*/main.go).
	for _, pattern := range entryPointGlobs {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			relPath, _ := filepath.Rel(root, match)
			if info.Size() > maxFileSize {
				s.filesSkipped++
				continue
			}
			s.filesRead++
			if detailLevel == "summary" {
				parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", relPath, info.Size()))
			} else {
				content := readFirstLines(match, maxEntryPointLines)
				if content != "" {
					parts = append(parts, fmt.Sprintf("### %s\n\n```go\n%s\n```", relPath, content))
				}
			}
		}
	}

	if len(parts) == 0 {
		s.content = "_No entry points found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- Convention scanner ---

// scanConventions reads convention files (CLAUDE.md, AGENTS.md, etc.).
// Reuses the conventionFiles and conventionDirs package vars from context_check.go.
func scanConventions(root, detailLevel string) scanSection {
	s := scanSection{title: "Conventions & Style"}
	var parts []string

	// Scan individual convention files.
	for _, filename := range conventionFiles {
		path := filepath.Join(root, filename)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			continue
		}
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", filename, info.Size()))
		} else {
			content := readFirstLines(path, maxConventionLines)
			if content != "" {
				parts = append(parts, fmt.Sprintf("### %s\n\n```markdown\n%s\n```", filename, content))
			}
		}
	}

	// Also check .github/copilot-instructions.md (not in conventionFiles list).
	copilotPath := filepath.Join(root, ".github", "copilot-instructions.md")
	if info, err := os.Stat(copilotPath); err == nil && info.Size() <= maxFileSize {
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", ".github/copilot-instructions.md", info.Size()))
		} else {
			content := readFirstLines(copilotPath, maxConventionLines)
			if content != "" {
				parts = append(parts, fmt.Sprintf("### .github/copilot-instructions.md\n\n```markdown\n%s\n```", content))
			}
		}
	}

	// Scan convention directories.
	for _, dir := range conventionDirs {
		dirPath := filepath.Join(root, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dirPath, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.Size() > maxFileSize {
				s.filesSkipped++
				continue
			}
			s.filesRead++
			relPath := filepath.Join(dir, entry.Name())
			if detailLevel == "summary" {
				parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", relPath, info.Size()))
			} else {
				content := readFirstLines(path, maxConventionLines)
				if content != "" {
					parts = append(parts, fmt.Sprintf("### %s\n\n```\n%s\n```", relPath, content))
				}
			}
		}
	}

	if len(parts) == 0 {
		s.content = "_No convention files found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- Schema scanner ---

// schemaDirs are directories to scan for database schemas.
var schemaDirs = []string{
	"migrations",
	"db/migrate",
	"db/migrations",
	"alembic/versions",
	"drizzle",
	"sql",
	"database/migrations",
	"src/migrations",
}

// schemaFiles are specific schema files to detect.
var schemaFiles = []string{
	"prisma/schema.prisma",
	"schema.prisma",
	"drizzle.config.ts",
	"knexfile.js",
	"knexfile.ts",
	"ormconfig.json",
	"ormconfig.ts",
	"typeorm.config.ts",
	"sequelize.config.js",
	"database.yml",
	"schema.rb",
	"structure.sql",
}

// scanSchemas detects and reads database schema/migration files.
func scanSchemas(root, detailLevel string) scanSection {
	s := scanSection{title: "Data Model Evidence"}
	var parts []string

	// Check schema directories — report file count and read latest files.
	for _, dir := range schemaDirs {
		dirPath := filepath.Join(root, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		var files []os.DirEntry
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, e)
			}
		}
		if len(files) == 0 {
			continue
		}

		parts = append(parts, fmt.Sprintf("### %s/ (%d files)", dir, len(files)))
		s.filesRead += len(files)

		if detailLevel != "summary" {
			// Read the last 3 files (most recent migrations).
			start := 0
			if len(files) > 3 {
				start = len(files) - 3
			}
			for _, f := range files[start:] {
				path := filepath.Join(dirPath, f.Name())
				info, err := f.Info()
				if err != nil {
					continue
				}
				if info.Size() > maxFileSize {
					s.filesSkipped++
					continue
				}
				content := readFirstLines(path, maxConfigLines)
				if content != "" {
					parts = append(parts, fmt.Sprintf("#### %s\n\n```sql\n%s\n```", f.Name(), content))
				}
			}
		}
	}

	// Check specific schema files.
	for _, name := range schemaFiles {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			continue
		}
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", name, info.Size()))
		} else {
			content := readFirstLines(path, maxConfigLines)
			if content != "" {
				ext := filepath.Ext(name)
				lang := langFromExt(ext)
				parts = append(parts, fmt.Sprintf("### %s\n\n```%s\n%s\n```", name, lang, content))
			}
		}
	}

	if len(parts) == 0 {
		s.content = "_No database schemas or migrations found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- API definition scanner ---

// apiSpecFiles are OpenAPI/Swagger spec files to detect.
var apiSpecFiles = []string{
	"openapi.yaml",
	"openapi.yml",
	"openapi.json",
	"swagger.yaml",
	"swagger.yml",
	"swagger.json",
	"api/openapi.yaml",
	"api/openapi.yml",
	"api/openapi.json",
	"docs/openapi.yaml",
}

// apiRoutePatterns are filename patterns that typically contain route definitions.
var apiRoutePatterns = []string{
	"routes.go",
	"routes.ts",
	"routes.js",
	"router.go",
	"router.ts",
	"router.js",
	"urls.py",
	"api.py",
}

// scanAPIDefs detects and reads API definition files.
func scanAPIDefs(root, detailLevel string) scanSection {
	s := scanSection{title: "API Evidence"}
	var parts []string

	// Check OpenAPI/Swagger specs.
	for _, name := range apiSpecFiles {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			parts = append(parts, fmt.Sprintf("- **%s** (skipped: %d bytes > 100KB)", name, info.Size()))
			continue
		}
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", name, info.Size()))
		} else {
			content := readFirstLines(path, maxConfigLines)
			if content != "" {
				ext := filepath.Ext(name)
				lang := langFromExt(ext)
				parts = append(parts, fmt.Sprintf("### %s\n\n```%s\n%s\n```", name, lang, content))
			}
		}
	}

	// Check route files at root level.
	for _, name := range apiRoutePatterns {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			s.filesSkipped++
			continue
		}
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", name, info.Size()))
		} else {
			content := readFirstLines(path, maxEntryPointLines)
			if content != "" {
				ext := filepath.Ext(name)
				lang := langFromExt(ext)
				parts = append(parts, fmt.Sprintf("### %s\n\n```%s\n%s\n```", name, lang, content))
			}
		}
	}

	// Walk for route files in subdirectories (1 level deep to avoid noise).
	routeNames := map[string]bool{
		"routes.go": true, "router.go": true,
		"routes.ts": true, "router.ts": true,
		"routes.js": true, "router.js": true,
		"urls.py": true, "api.py": true,
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if strings.Count(rel, string(filepath.Separator)) > 2 {
				return filepath.SkipDir
			}
			return nil
		}
		if !routeNames[d.Name()] {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		// Skip if already handled at root level.
		if !strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			s.filesSkipped++
			return nil
		}
		s.filesRead++
		if detailLevel == "summary" {
			parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", rel, info.Size()))
		} else {
			content := readFirstLines(path, maxEntryPointLines)
			if content != "" {
				ext := filepath.Ext(rel)
				lang := langFromExt(ext)
				parts = append(parts, fmt.Sprintf("### %s\n\n```%s\n%s\n```", rel, lang, content))
			}
		}
		return nil
	})

	if len(parts) == 0 {
		s.content = "_No API definitions or route files found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- ADR scanner ---

// adrDirs are directories to scan for Architecture Decision Records.
var adrDirs = []string{
	"docs/adrs",
	"docs/adr",
	"adr",
	"doc/decisions",
	"docs/decisions",
	"architectural-decisions",
	"docs/architecture",
	"docs/changes",
}

// scanADRs detects and reads ADR files.
func scanADRs(root, detailLevel string) scanSection {
	s := scanSection{title: "Prior Decisions"}
	var parts []string

	for _, dir := range adrDirs {
		dirPath := filepath.Join(root, dir)
		if _, err := os.Stat(dirPath); err != nil {
			continue
		}

		// Walk the ADR directory (may have subdirectories for change pipeline).
		_ = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := filepath.Ext(d.Name())
			if ext != ".md" && ext != ".txt" && ext != ".rst" {
				return nil
			}
			// Only read files that look like ADRs.
			nameLower := strings.ToLower(d.Name())
			if !strings.Contains(nameLower, "adr") &&
				!strings.Contains(nameLower, "decision") &&
				!strings.HasPrefix(nameLower, "0") { // 001-*, 0001-*
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}
			relPath, _ := filepath.Rel(root, path)
			if info.Size() > maxFileSize {
				s.filesSkipped++
				return nil
			}
			s.filesRead++
			if detailLevel == "summary" {
				parts = append(parts, fmt.Sprintf("- **%s** (%d bytes)", relPath, info.Size()))
			} else {
				// ADRs are typically short — read in full.
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				parts = append(parts, fmt.Sprintf("### %s\n\n%s", relPath, string(data)))
			}
			return nil
		})
	}

	if len(parts) == 0 {
		s.content = "_No ADR files found._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- Test scanner ---

// testDirNames are directory names that typically contain tests.
var testDirNames = map[string]bool{
	"test": true, "tests": true, "__tests__": true,
	"spec": true, "specs": true, "e2e": true,
	"integration": true, "unit": true,
}

// testFrameworkIndicators maps file patterns to test framework names.
var testFrameworkIndicators = map[string]string{
	"jest.config.js":       "Jest",
	"jest.config.ts":       "Jest",
	"jest.config.mjs":      "Jest",
	"vitest.config.ts":     "Vitest",
	"vitest.config.js":     "Vitest",
	"vitest.config.mts":    "Vitest",
	"cypress.config.js":    "Cypress",
	"cypress.config.ts":    "Cypress",
	"playwright.config.ts": "Playwright",
	"playwright.config.js": "Playwright",
	".mocharc.yml":         "Mocha",
	".mocharc.json":        "Mocha",
	"pytest.ini":           "pytest",
	"setup.cfg":            "pytest", // may contain [tool:pytest]
	"conftest.py":          "pytest",
	"phpunit.xml":          "PHPUnit",
	"phpunit.xml.dist":     "PHPUnit",
	".rspec":               "RSpec",
	"spec_helper.rb":       "RSpec",
}

// testFilePatterns maps suffixes to framework detection.
var testFilePatterns = map[string]string{
	"_test.go":  "Go testing",
	".test.ts":  "TypeScript (Jest/Vitest)",
	".test.tsx": "TypeScript (Jest/Vitest)",
	".test.js":  "JavaScript (Jest/Vitest)",
	".test.jsx": "JavaScript (Jest/Vitest)",
	".spec.ts":  "TypeScript (Jest/Vitest)",
	".spec.tsx": "TypeScript (Jest/Vitest)",
	".spec.js":  "JavaScript (Jest/Vitest)",
	".spec.jsx": "JavaScript (Jest/Vitest)",
	"_test.py":  "pytest",
	"test_":     "pytest",
	"_spec.rb":  "RSpec",
}

// scanTests detects test directories, frameworks, and approximate file counts.
func scanTests(root, detailLevel string) scanSection {
	s := scanSection{title: "Test Evidence"}
	var parts []string

	// Detect test frameworks from config files.
	var frameworks []string
	frameworkSeen := map[string]bool{}
	for file, framework := range testFrameworkIndicators {
		path := filepath.Join(root, file)
		if _, err := os.Stat(path); err == nil {
			if !frameworkSeen[framework] {
				frameworks = append(frameworks, framework)
				frameworkSeen[framework] = true
			}
		}
	}

	// Count test files by walking.
	testFileCount := 0
	testDirsSeen := map[string]bool{}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if testDirNames[d.Name()] {
				testDirsSeen[rel] = true
			}
			return nil
		}

		// Check if file matches test patterns.
		name := d.Name()
		for suffix, framework := range testFilePatterns {
			if strings.HasSuffix(name, suffix) || (suffix == "test_" && strings.HasPrefix(name, "test_")) {
				testFileCount++
				if !frameworkSeen[framework] {
					frameworks = append(frameworks, framework)
					frameworkSeen[framework] = true
				}
				break
			}
		}
		return nil
	})

	// Sort frameworks for deterministic output.
	sort.Strings(frameworks)

	if len(frameworks) > 0 {
		parts = append(parts, fmt.Sprintf("**Frameworks detected**: %s", strings.Join(frameworks, ", ")))
	}
	if testFileCount > 0 {
		parts = append(parts, fmt.Sprintf("**Test files found**: %d", testFileCount))
	}
	if len(testDirsSeen) > 0 {
		dirs := make([]string, 0, len(testDirsSeen))
		for d := range testDirsSeen {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		parts = append(parts, fmt.Sprintf("**Test directories**: %s", strings.Join(dirs, ", ")))
	}

	if len(parts) == 0 {
		s.content = "_No test files or frameworks detected._"
	} else {
		s.content = strings.Join(parts, "\n\n")
	}
	return s
}

// --- Ecosystem detection ---

// detectEcosystem infers the primary language/framework from manifests.
func detectEcosystem(root string) string {
	checks := []struct {
		file      string
		ecosystem string
	}{
		{"go.mod", "Go"},
		{"package.json", "Node.js"},
		{"pyproject.toml", "Python"},
		{"requirements.txt", "Python"},
		{"Cargo.toml", "Rust"},
		{"pom.xml", "Java"},
		{"build.gradle", "Java"},
		{"build.gradle.kts", "Kotlin"},
		{"Gemfile", "Ruby"},
		{"composer.json", "PHP"},
		{"mix.exs", "Elixir"},
		{"pubspec.yaml", "Dart/Flutter"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(root, c.file)); err == nil {
			return c.ecosystem
		}
	}
	return "Unknown"
}

// --- Monorepo detection ---

// detectMonorepo checks for workspace/monorepo patterns.
func detectMonorepo(root string) []string {
	var workspaces []string

	// Check pnpm-workspace.yaml.
	if _, err := os.Stat(filepath.Join(root, "pnpm-workspace.yaml")); err == nil {
		workspaces = append(workspaces, "pnpm workspaces")
	}

	// Check lerna.json.
	if _, err := os.Stat(filepath.Join(root, "lerna.json")); err == nil {
		workspaces = append(workspaces, "Lerna")
	}

	// Check nx.json.
	if _, err := os.Stat(filepath.Join(root, "nx.json")); err == nil {
		workspaces = append(workspaces, "Nx")
	}

	// Check turbo.json.
	if _, err := os.Stat(filepath.Join(root, "turbo.json")); err == nil {
		workspaces = append(workspaces, "Turborepo")
	}

	// Check standard monorepo directories.
	monoDirs := []string{"packages", "apps", "services", "libs"}
	for _, d := range monoDirs {
		dirPath := filepath.Join(root, d)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		var subPackages []string
		for _, e := range entries {
			if e.IsDir() {
				subPackages = append(subPackages, e.Name())
			}
		}
		if len(subPackages) > 0 {
			workspaces = append(workspaces, fmt.Sprintf("%s/ (%s)", d, strings.Join(subPackages, ", ")))
		}
	}

	return workspaces
}

// --- Helpers ---

// --- MCP Tool handler ---

// ReverseEngineerTool handles the sdd_reverse_engineer MCP tool.
// It orchestrates all sub-scanners, assembles a structured markdown report,
// and applies token budgeting. Read-only — never writes files.
type ReverseEngineerTool struct{}

// NewReverseEngineerTool creates a ReverseEngineerTool.
// No dependencies — pure filesystem scanner.
func NewReverseEngineerTool() *ReverseEngineerTool {
	return &ReverseEngineerTool{}
}

// Definition returns the MCP tool definition for registration.
func (t *ReverseEngineerTool) Definition() mcp.Tool {
	return mcp.NewTool("sdd_reverse_engineer",
		mcp.WithDescription(
			"Scan an existing project's filesystem and produce a structured markdown report "+
				"for artifact generation. This is a READ-ONLY scanner — it never writes files. "+
				"Use the scan report to understand the project's architecture, then call "+
				"`sdd_bootstrap` to generate missing SDD artifacts (business-rules.md, "+
				"design.md, requirements.md). "+
				"Use this for projects that weren't created through the SDD pipeline.",
		),
		mcp.WithString("detail_level",
			mcp.Description("Verbosity of the scan report: 'summary' (filenames only), "+
				"'standard' (default — truncated content), 'full' (complete file contents)."),
			mcp.Enum(memory.DetailLevelValues()...),
		),
		mcp.WithNumber("max_tokens",
			mcp.Description("Token budget cap. When set, truncates sections to stay within budget. "+
				"Sections are prioritized: overview first, ADRs last. 0 = no cap."),
		),
		mcp.WithString("scan_path",
			mcp.Description("Subdirectory to scan instead of project root. "+
				"Useful for monorepos where you want to scan a specific package."),
		),
		mcp.WithNumber("max_depth",
			mcp.Description("Maximum directory tree depth (default: 3). "+
				"Increase for deeply nested projects, decrease for flatter ones."),
		),
	)
}

// Handle processes the sdd_reverse_engineer tool call.
func (t *ReverseEngineerTool) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	detailLevel := memory.ParseDetailLevel(req.GetString("detail_level", ""))
	maxTokens := int(req.GetFloat("max_tokens", 0))
	scanPath := req.GetString("scan_path", "")
	maxDepth := int(req.GetFloat("max_depth", 3))

	if maxDepth <= 0 {
		maxDepth = 3
	}

	// Resolve scan root.
	root, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("finding project root: %w", err)
	}
	if scanPath != "" {
		candidate := filepath.Join(root, scanPath)
		info, err := os.Stat(candidate)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan_path '%s' not found: %v", scanPath, err)), nil
		}
		if !info.IsDir() {
			return mcp.NewToolResultError(fmt.Sprintf("scan_path '%s' is not a directory", scanPath)), nil
		}
		root = candidate
	}

	start := time.Now()

	// Run all sub-scanners sequentially.
	sections := []scanSection{
		scanManifests(root, detailLevel),
		scanStructure(root, detailLevel, maxDepth),
		scanConfigs(root, detailLevel),
		scanEntryPoints(root, detailLevel),
		scanConventions(root, detailLevel),
		scanSchemas(root, detailLevel),
		scanAPIDefs(root, detailLevel),
		scanADRs(root, detailLevel),
		scanTests(root, detailLevel),
	}

	duration := time.Since(start)

	// Collect totals.
	totalRead := 0
	totalSkipped := 0
	for _, s := range sections {
		totalRead += s.filesRead
		totalSkipped += s.filesSkipped
	}

	ecosystem := detectEcosystem(root)
	monorepo := detectMonorepo(root)

	// Build report.
	var report strings.Builder

	// AI instruction block (FR-016).
	report.WriteString("# Project Scan Report\n\n")
	report.WriteString("> **AI Instructions**: Analyze this scan report and generate content for ")
	report.WriteString("the missing SDD artifacts. Call `sdd_bootstrap` with the generated content.\n")
	report.WriteString("> Only generate artifacts that don't already exist in `docs/`.\n")
	report.WriteString("> Focus on: business rules (domain terms, facts, constraints), ")
	report.WriteString("requirements (functional and non-functional), and design (architecture, tech stack, components).\n\n")

	// Metadata header.
	report.WriteString("## Scan Metadata\n\n")
	fmt.Fprintf(&report, "- **Project root**: `%s`\n", root)
	fmt.Fprintf(&report, "- **Primary ecosystem**: %s\n", ecosystem)
	if len(monorepo) > 0 {
		fmt.Fprintf(&report, "- **Monorepo**: %s\n", strings.Join(monorepo, ", "))
	}
	fmt.Fprintf(&report, "- **Files scanned**: %d\n", totalRead)
	if totalSkipped > 0 {
		fmt.Fprintf(&report, "- **Files skipped**: %d\n", totalSkipped)
	}
	fmt.Fprintf(&report, "- **Scan duration**: %s\n", duration.Round(time.Millisecond))
	fmt.Fprintf(&report, "- **Detail level**: %s\n\n", detailLevel)

	// Check existing artifacts.
	hasReqs := ArtifactExists(root, "specify")
	hasRules := ArtifactExists(root, "business-rules")
	hasDesign := ArtifactExists(root, "design")

	if hasReqs || hasRules || hasDesign {
		report.WriteString("## Existing SDD Artifacts\n\n")
		if hasReqs {
			report.WriteString("- ✅ `docs/requirements.md` — already exists\n")
		}
		if hasRules {
			report.WriteString("- ✅ `docs/business-rules.md` — already exists\n")
		}
		if hasDesign {
			report.WriteString("- ✅ `docs/design.md` — already exists\n")
		}
		missing := 0
		if !hasReqs {
			missing++
		}
		if !hasRules {
			missing++
		}
		if !hasDesign {
			missing++
		}
		if missing > 0 {
			fmt.Fprintf(&report, "\n⚠️ **%d artifact(s) missing** — generate only the missing ones.\n\n", missing)
		} else {
			report.WriteString("\n✅ All artifacts exist. No bootstrap needed.\n\n")
		}
	}

	// Append each section.
	for _, s := range sections {
		fmt.Fprintf(&report, "## %s\n\n", s.title)
		report.WriteString(s.content)
		report.WriteString("\n\n")
	}

	result := report.String()

	// Apply token budgeting if requested.
	if maxTokens > 0 {
		tokens := memory.EstimateTokens(result)
		if tokens > maxTokens {
			// Truncate from the bottom (lower-priority sections get cut).
			result = truncateReportToTokens(result, maxTokens)
			result += fmt.Sprintf("\n\n---\n⚡ Report truncated to ~%d token budget. Use higher max_tokens or detail_level=summary for more.",
				maxTokens)
		}
	}

	// Append token footer.
	tokens := memory.EstimateTokens(result)
	result += memory.TokenFooter(tokens)

	return mcp.NewToolResultText(result), nil
}

// truncateReportToTokens truncates the report by removing sections from
// the bottom until it fits within the token budget. Sections at the top
// (metadata, overview) have highest priority.
func truncateReportToTokens(report string, maxTokens int) string {
	// Split by section headers.
	parts := strings.Split(report, "\n## ")
	if len(parts) <= 1 {
		// No sections to truncate — just cut the string.
		cutAt := maxTokens * 4 // rough chars from tokens
		if cutAt >= len(report) {
			return report
		}
		return report[:cutAt] + "\n\n_[truncated]_"
	}

	// Rebuild from top, adding sections until budget exceeded.
	var result strings.Builder
	result.WriteString(parts[0]) // everything before first ##
	tokensUsed := memory.EstimateTokens(parts[0])

	for i := 1; i < len(parts); i++ {
		section := "\n## " + parts[i]
		sectionTokens := memory.EstimateTokens(section)
		if tokensUsed+sectionTokens > maxTokens {
			result.WriteString("\n\n_[remaining sections truncated due to token budget]_")
			break
		}
		result.WriteString(section)
		tokensUsed += sectionTokens
	}

	return result.String()
}

// langFromExt maps file extensions to code block language hints.
func langFromExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".java":
		return "java"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".sql":
		return "sql"
	case ".prisma":
		return "prisma"
	default:
		return ""
	}
}
