package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// --- Test helpers for reverse engineer scanners ---

// setupGoProject creates a temp directory that looks like a Go project.
func setupGoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// go.mod
	writeTestFile(t, dir, "go.mod", "module example.com/myapp\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.0\n)\n")

	// main.go
	writeTestFile(t, dir, "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")

	// cmd/api/main.go
	writeTestFile(t, dir, "cmd/api/main.go", "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.ListenAndServe(\":8080\", nil)\n}\n")

	// Makefile
	writeTestFile(t, dir, "Makefile", "build:\n\tgo build -o bin/app ./cmd/api\n\ntest:\n\tgo test ./...\n")

	// internal directory
	writeTestFile(t, dir, "internal/handler/routes.go", "package handler\n\nfunc SetupRoutes() {}\n")
	writeTestFile(t, dir, "internal/handler/routes_test.go", "package handler\n\nfunc TestSetupRoutes(t *testing.T) {}\n")
	writeTestFile(t, dir, "internal/model/user.go", "package model\n\ntype User struct { ID int }\n")

	// AGENTS.md
	writeTestFile(t, dir, "AGENTS.md", "# AGENTS.md\n\n## Rules\n- Use conventional commits\n")

	// migrations
	writeTestFile(t, dir, "migrations/001_create_users.sql", "CREATE TABLE users (id SERIAL PRIMARY KEY, email TEXT);\n")
	writeTestFile(t, dir, "migrations/002_add_name.sql", "ALTER TABLE users ADD COLUMN name TEXT;\n")

	return dir
}

// setupNodeProject creates a temp directory that looks like a Node.js project.
func setupNodeProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// package.json
	writeTestFile(t, dir, "package.json", `{"name": "my-app", "version": "1.0.0", "scripts": {"dev": "next dev"}, "dependencies": {"react": "^18.0.0", "next": "^14.0.0"}}`)

	// tsconfig.json
	writeTestFile(t, dir, "tsconfig.json", `{"compilerOptions": {"target": "ES2017", "strict": true}}`)

	// src/index.ts
	writeTestFile(t, dir, "src/index.ts", "import express from 'express';\n\nconst app = express();\napp.listen(3000);\n")

	// jest.config.js
	writeTestFile(t, dir, "jest.config.js", "module.exports = { preset: 'ts-jest' };\n")

	// src/routes.ts
	writeTestFile(t, dir, "src/routes.ts", "import { Router } from 'express';\nconst router = Router();\nexport default router;\n")

	// __tests__
	writeTestFile(t, dir, "__tests__/app.test.ts", "describe('app', () => { it('works', () => {}) });\n")

	// prisma
	writeTestFile(t, dir, "prisma/schema.prisma", "model User {\n  id Int @id\n  email String @unique\n}\n")

	return dir
}

// setupPythonProject creates a temp directory that looks like a Python project.
func setupPythonProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// pyproject.toml
	writeTestFile(t, dir, "pyproject.toml", "[project]\nname = \"my-app\"\nversion = \"0.1.0\"\n\n[tool.pytest]\ntestpaths = [\"tests\"]\n")

	// manage.py
	writeTestFile(t, dir, "manage.py", "#!/usr/bin/env python\nimport os\nimport sys\n\ndef main():\n    pass\n")

	// app.py
	writeTestFile(t, dir, "app.py", "from flask import Flask\napp = Flask(__name__)\n")

	// urls.py
	writeTestFile(t, dir, "urls.py", "urlpatterns = [\n    path('api/', include('api.urls')),\n]\n")

	// conftest.py
	writeTestFile(t, dir, "conftest.py", "import pytest\n")

	// tests/
	writeTestFile(t, dir, "tests/test_app.py", "def test_app():\n    assert True\n")

	return dir
}

// setupEmptyProject creates a bare minimum temp directory.
func setupEmptyProject(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// writeTestFile creates a file with the given content, creating parent dirs.
func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("writeTestFile: mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile: write: %v", err)
	}
}

// --- scanManifests tests ---

func TestScanManifests_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanManifests(root, "standard")

	if s.filesRead == 0 {
		t.Error("should read at least go.mod")
	}
	if !strings.Contains(s.content, "go.mod") {
		t.Error("should mention go.mod")
	}
	if !strings.Contains(s.content, "example.com/myapp") {
		t.Error("should include go.mod content")
	}
}

func TestScanManifests_NodeProject(t *testing.T) {
	root := setupNodeProject(t)
	s := scanManifests(root, "standard")

	if !strings.Contains(s.content, "package.json") {
		t.Error("should mention package.json")
	}
	if !strings.Contains(s.content, "my-app") {
		t.Error("should include package.json content")
	}
}

func TestScanManifests_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanManifests(root, "standard")

	if s.filesRead != 0 {
		t.Errorf("filesRead = %d, want 0", s.filesRead)
	}
	if !strings.Contains(s.content, "No package manifests") {
		t.Error("should say no manifests found")
	}
}

func TestScanManifests_SummaryLevel(t *testing.T) {
	root := setupGoProject(t)
	s := scanManifests(root, "summary")

	// Summary should NOT include file content, just names/sizes.
	if strings.Contains(s.content, "example.com/myapp") {
		t.Error("summary should not include file content")
	}
	if !strings.Contains(s.content, "go.mod") {
		t.Error("summary should list the filename")
	}
	if !strings.Contains(s.content, "bytes") {
		t.Error("summary should include size")
	}
}

// --- scanStructure tests ---

func TestScanStructure_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanStructure(root, "standard", 3)

	if !strings.Contains(s.content, "internal/") {
		t.Error("should show internal/ directory")
	}
	if !strings.Contains(s.content, "cmd/") {
		t.Error("should show cmd/ directory")
	}
}

func TestScanStructure_DepthLimit(t *testing.T) {
	root := setupGoProject(t)
	s := scanStructure(root, "standard", 1)

	// With depth 1, should show top-level dirs but not files inside them.
	if !strings.Contains(s.content, "internal/") {
		t.Error("should show internal/ at depth 1")
	}
	// Files inside internal/ should NOT appear with depth=1.
	if strings.Contains(s.content, "handler/") {
		t.Error("handler/ should not appear with depth=1 (it's at depth 2)")
	}
}

func TestScanStructure_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanStructure(root, "standard", 3)

	if !strings.Contains(s.content, "```") {
		t.Error("should contain code fence")
	}
}

// --- scanConfigs tests ---

func TestScanConfigs_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanConfigs(root, "standard")

	if !strings.Contains(s.content, "Makefile") {
		t.Error("should detect Makefile")
	}
}

func TestScanConfigs_NodeProject(t *testing.T) {
	root := setupNodeProject(t)
	s := scanConfigs(root, "standard")

	if !strings.Contains(s.content, "tsconfig.json") {
		t.Error("should detect tsconfig.json")
	}
}

func TestScanConfigs_CIFiles(t *testing.T) {
	root := setupGoProject(t)
	writeTestFile(t, root, ".github/workflows/ci.yml", "name: CI\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n")

	s := scanConfigs(root, "standard")
	if !strings.Contains(s.content, "ci.yml") {
		t.Error("should detect CI workflow files")
	}
}

func TestScanConfigs_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanConfigs(root, "standard")

	if !strings.Contains(s.content, "No configuration files") {
		t.Error("should say no configs found")
	}
}

// --- scanEntryPoints tests ---

func TestScanEntryPoints_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanEntryPoints(root, "standard")

	if !strings.Contains(s.content, "main.go") {
		t.Error("should detect main.go")
	}
	if !strings.Contains(s.content, "cmd/api/main.go") {
		t.Error("should detect cmd/api/main.go via glob")
	}
}

func TestScanEntryPoints_NodeProject(t *testing.T) {
	root := setupNodeProject(t)
	s := scanEntryPoints(root, "standard")

	if !strings.Contains(s.content, "src/index.ts") {
		t.Error("should detect src/index.ts")
	}
}

func TestScanEntryPoints_PythonProject(t *testing.T) {
	root := setupPythonProject(t)
	s := scanEntryPoints(root, "standard")

	if !strings.Contains(s.content, "manage.py") {
		t.Error("should detect manage.py")
	}
	if !strings.Contains(s.content, "app.py") {
		t.Error("should detect app.py")
	}
}

func TestScanEntryPoints_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanEntryPoints(root, "standard")

	if !strings.Contains(s.content, "No entry points") {
		t.Error("should say no entry points found")
	}
}

// --- scanConventions tests ---

func TestScanConventions_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanConventions(root, "standard")

	if !strings.Contains(s.content, "AGENTS.md") {
		t.Error("should detect AGENTS.md")
	}
	if !strings.Contains(s.content, "conventional commits") {
		t.Error("should include AGENTS.md content")
	}
}

func TestScanConventions_CopilotInstructions(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, ".github/copilot-instructions.md", "# Copilot Instructions\nUse TypeScript.\n")

	s := scanConventions(root, "standard")
	if !strings.Contains(s.content, "copilot-instructions.md") {
		t.Error("should detect .github/copilot-instructions.md")
	}
}

func TestScanConventions_CursorRules(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, ".cursor/rules/typescript.md", "# TypeScript Rules\nAlways use strict mode.\n")

	s := scanConventions(root, "standard")
	if !strings.Contains(s.content, "typescript.md") {
		t.Error("should detect .cursor/rules/ files")
	}
}

func TestScanConventions_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanConventions(root, "standard")

	if !strings.Contains(s.content, "No convention files") {
		t.Error("should say no convention files found")
	}
}

// --- scanSchemas tests ---

func TestScanSchemas_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanSchemas(root, "standard")

	if !strings.Contains(s.content, "migrations/") {
		t.Error("should detect migrations directory")
	}
	if !strings.Contains(s.content, "CREATE TABLE") {
		t.Error("should include migration content")
	}
}

func TestScanSchemas_Prisma(t *testing.T) {
	root := setupNodeProject(t)
	s := scanSchemas(root, "standard")

	if !strings.Contains(s.content, "prisma/schema.prisma") {
		t.Error("should detect prisma schema")
	}
	if !strings.Contains(s.content, "model User") {
		t.Error("should include prisma content")
	}
}

func TestScanSchemas_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanSchemas(root, "standard")

	if !strings.Contains(s.content, "No database schemas") {
		t.Error("should say no schemas found")
	}
}

// --- scanAPIDefs tests ---

func TestScanAPIDefs_OpenAPI(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, "openapi.yaml", "openapi: 3.0.0\ninfo:\n  title: My API\n  version: 1.0.0\npaths: {}\n")

	s := scanAPIDefs(root, "standard")
	if !strings.Contains(s.content, "openapi.yaml") {
		t.Error("should detect openapi.yaml")
	}
	if !strings.Contains(s.content, "My API") {
		t.Error("should include OpenAPI content")
	}
}

func TestScanAPIDefs_RouteFiles(t *testing.T) {
	root := setupGoProject(t)
	s := scanAPIDefs(root, "standard")

	// internal/handler/routes.go should be found by the walker.
	if !strings.Contains(s.content, "routes.go") {
		t.Error("should detect route files")
	}
}

func TestScanAPIDefs_PythonURLs(t *testing.T) {
	root := setupPythonProject(t)
	s := scanAPIDefs(root, "standard")

	if !strings.Contains(s.content, "urls.py") {
		t.Error("should detect urls.py")
	}
}

func TestScanAPIDefs_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanAPIDefs(root, "standard")

	if !strings.Contains(s.content, "No API definitions") {
		t.Error("should say no API defs found")
	}
}

// --- scanADRs tests ---

func TestScanADRs_WithADRDir(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, "docs/adr/ADR-001-use-postgres.md", "# ADR-001: Use PostgreSQL\n\n## Decision\nUse PostgreSQL for the database.\n")

	s := scanADRs(root, "standard")
	if !strings.Contains(s.content, "ADR-001") {
		t.Error("should detect ADR files")
	}
	if !strings.Contains(s.content, "PostgreSQL") {
		t.Error("should include ADR content")
	}
}

func TestScanADRs_NumberedFiles(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, "adr/0001-initial-architecture.md", "# Initial Architecture\nMonolith first.\n")

	s := scanADRs(root, "standard")
	if !strings.Contains(s.content, "0001-initial-architecture.md") {
		t.Error("should detect numbered ADR files")
	}
}

func TestScanADRs_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanADRs(root, "standard")

	if !strings.Contains(s.content, "No ADR files") {
		t.Error("should say no ADRs found")
	}
}

// --- scanTests tests ---

func TestScanTests_GoProject(t *testing.T) {
	root := setupGoProject(t)
	s := scanTests(root, "standard")

	if !strings.Contains(s.content, "Go testing") {
		t.Error("should detect Go testing framework")
	}
	if !strings.Contains(s.content, "Test files found") {
		t.Error("should report test file count")
	}
}

func TestScanTests_NodeProject(t *testing.T) {
	root := setupNodeProject(t)
	s := scanTests(root, "standard")

	if !strings.Contains(s.content, "Jest") {
		t.Error("should detect Jest from jest.config.js")
	}
	if !strings.Contains(s.content, "Test files found") {
		t.Error("should report test file count")
	}
}

func TestScanTests_PythonProject(t *testing.T) {
	root := setupPythonProject(t)
	s := scanTests(root, "standard")

	if !strings.Contains(s.content, "pytest") {
		t.Error("should detect pytest from conftest.py")
	}
}

func TestScanTests_EmptyProject(t *testing.T) {
	root := setupEmptyProject(t)
	s := scanTests(root, "standard")

	if !strings.Contains(s.content, "No test files") {
		t.Error("should say no test files found")
	}
}

// --- detectEcosystem tests ---

func TestDetectEcosystem(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) string
		want  string
	}{
		{"Go", setupGoProject, "Go"},
		{"Node.js", setupNodeProject, "Node.js"},
		{"Python", setupPythonProject, "Python"},
		{"Empty", setupEmptyProject, "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setup(t)
			got := detectEcosystem(root)
			if got != tt.want {
				t.Errorf("detectEcosystem = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- detectMonorepo tests ---

func TestDetectMonorepo_Pnpm(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, "pnpm-workspace.yaml", "packages:\n  - packages/*\n")

	ws := detectMonorepo(root)
	if len(ws) == 0 {
		t.Error("should detect pnpm workspace")
	}
}

func TestDetectMonorepo_PackagesDir(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, "packages/ui/.gitkeep", "")
	writeTestFile(t, root, "packages/api/.gitkeep", "")

	ws := detectMonorepo(root)
	found := false
	for _, w := range ws {
		if strings.Contains(w, "packages/") {
			found = true
		}
	}
	if !found {
		t.Error("should detect packages/ monorepo structure")
	}
}

func TestDetectMonorepo_NoMonorepo(t *testing.T) {
	root := setupEmptyProject(t)
	ws := detectMonorepo(root)
	if len(ws) != 0 {
		t.Errorf("should not detect monorepo, got: %v", ws)
	}
}

// --- langFromExt tests ---

func TestLangFromExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".go", "go"},
		{".ts", "typescript"},
		{".py", "python"},
		{".rs", "rust"},
		{".json", "json"},
		{".yaml", "yaml"},
		{".unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := langFromExt(tt.ext)
			if got != tt.want {
				t.Errorf("langFromExt(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

// --- File size guard tests ---

func TestScanManifests_SkipsLargeFiles(t *testing.T) {
	root := setupEmptyProject(t)
	// Create a package.json larger than 100KB.
	largeContent := strings.Repeat("x", maxFileSize+1)
	writeTestFile(t, root, "package.json", largeContent)

	s := scanManifests(root, "standard")
	if s.filesSkipped != 1 {
		t.Errorf("filesSkipped = %d, want 1", s.filesSkipped)
	}
	if s.filesRead != 0 {
		t.Errorf("filesRead = %d, want 0", s.filesRead)
	}
}

// --- Graceful degradation tests ---

func TestScanStructure_IgnoresDirs(t *testing.T) {
	root := setupEmptyProject(t)
	writeTestFile(t, root, "src/app.ts", "const x = 1;\n")
	writeTestFile(t, root, "node_modules/lodash/index.js", "module.exports = {};\n")
	writeTestFile(t, root, ".git/config", "[core]\n")

	s := scanStructure(root, "standard", 3)
	if strings.Contains(s.content, "node_modules") {
		t.Error("should not include node_modules")
	}
	if strings.Contains(s.content, ".git") {
		t.Error("should not include .git")
	}
	if !strings.Contains(s.content, "src/") {
		t.Error("should include src/")
	}
}

// --- ReverseEngineerTool handler tests ---

// setupHandlerProject creates a project, changes cwd to it, and returns
// cleanup. The handler uses findProjectRoot() which checks cwd.
func setupHandlerProject(t *testing.T, setupFn func(*testing.T) string) (string, func()) {
	t.Helper()
	root := setupFn(t)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("setup: getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("setup: chdir: %v", err)
	}
	cleanup := func() { _ = os.Chdir(origDir) }
	return root, cleanup
}

func TestReverseEngineerTool_Handle_GoProject(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupGoProject)
	defer cleanup()

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)

	// Report header.
	if !strings.Contains(text, "# Project Scan Report") {
		t.Error("should have report title")
	}
	if !strings.Contains(text, "AI Instructions") {
		t.Error("should have AI instruction block")
	}

	// Metadata.
	if !strings.Contains(text, "Primary ecosystem") {
		t.Error("should have ecosystem in metadata")
	}
	if !strings.Contains(text, "Go") {
		t.Error("should detect Go ecosystem")
	}
	if !strings.Contains(text, "Files scanned") {
		t.Error("should report files scanned")
	}

	// All 9 sections should appear.
	sections := []string{
		"Project Overview",
		"Directory Structure",
		"Tech Stack Evidence",
		"Architecture Evidence",
		"Conventions & Style",
		"Data Model Evidence",
		"API Evidence",
		"Prior Decisions",
		"Test Evidence",
	}
	for _, s := range sections {
		if !strings.Contains(text, s) {
			t.Errorf("report should contain section %q", s)
		}
	}
}

func TestReverseEngineerTool_Handle_NodeProject(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupNodeProject)
	defer cleanup()

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Node.js") {
		t.Error("should detect Node.js ecosystem")
	}
	if !strings.Contains(text, "package.json") {
		t.Error("should include package.json in manifests")
	}
}

func TestReverseEngineerTool_Handle_EmptyProject(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupEmptyProject)
	defer cleanup()

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "Unknown") {
		t.Error("should detect Unknown ecosystem for empty project")
	}
	if !strings.Contains(text, "# Project Scan Report") {
		t.Error("should still produce a report")
	}
}

func TestReverseEngineerTool_Handle_ScanPath(t *testing.T) {
	root, cleanup := setupHandlerProject(t, setupEmptyProject)
	defer cleanup()

	// Create a subdirectory with a Go project inside.
	subDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeTestFile(t, root, "services/api/go.mod", "module example.com/api\n\ngo 1.21\n")
	writeTestFile(t, root, "services/api/main.go", "package main\n\nfunc main() {}\n")

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"scan_path": "services/api",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}

	text := getResultText(result)
	if !strings.Contains(text, "go.mod") {
		t.Error("should scan the subdirectory and find go.mod")
	}
}

func TestReverseEngineerTool_Handle_ScanPath_Invalid(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupEmptyProject)
	defer cleanup()

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"scan_path": "nonexistent/path",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error for nonexistent scan_path")
	}
	text := getResultText(result)
	if !strings.Contains(text, "not found") {
		t.Errorf("error should mention 'not found': %s", text)
	}
}

func TestReverseEngineerTool_Handle_ScanPath_NotDir(t *testing.T) {
	root, cleanup := setupHandlerProject(t, setupEmptyProject)
	defer cleanup()

	writeTestFile(t, root, "file.txt", "hello")

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"scan_path": "file.txt",
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !isErrorResult(result) {
		t.Error("should return error when scan_path is a file, not dir")
	}
	text := getResultText(result)
	if !strings.Contains(text, "not a directory") {
		t.Errorf("error should mention 'not a directory': %s", text)
	}
}

func TestReverseEngineerTool_Handle_DetailLevel_Summary(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupGoProject)
	defer cleanup()

	tool := NewReverseEngineerTool()

	// Standard scan for comparison.
	reqStd := mcp.CallToolRequest{}
	reqStd.Params.Arguments = map[string]interface{}{"detail_level": "standard"}
	resultStd, _ := tool.Handle(context.Background(), reqStd)
	textStd := getResultText(resultStd)

	// Summary scan.
	reqSum := mcp.CallToolRequest{}
	reqSum.Params.Arguments = map[string]interface{}{"detail_level": "summary"}
	resultSum, _ := tool.Handle(context.Background(), reqSum)
	textSum := getResultText(resultSum)

	// Summary should be shorter than standard.
	if len(textSum) >= len(textStd) {
		t.Error("summary report should be shorter than standard report")
	}

	// Summary should still have metadata.
	if !strings.Contains(textSum, "summary") {
		t.Error("summary should indicate detail level in metadata")
	}
}

func TestReverseEngineerTool_Handle_MaxTokens(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupGoProject)
	defer cleanup()

	tool := NewReverseEngineerTool()

	// Full scan without budget.
	reqFull := mcp.CallToolRequest{}
	reqFull.Params.Arguments = map[string]interface{}{}
	resultFull, _ := tool.Handle(context.Background(), reqFull)
	textFull := getResultText(resultFull)

	// Scan with tiny budget to force truncation.
	reqTrunc := mcp.CallToolRequest{}
	reqTrunc.Params.Arguments = map[string]interface{}{
		"max_tokens": float64(100),
	}
	resultTrunc, _ := tool.Handle(context.Background(), reqTrunc)
	textTrunc := getResultText(resultTrunc)

	if len(textTrunc) >= len(textFull) {
		t.Error("truncated report should be shorter than full report")
	}
	if !strings.Contains(textTrunc, "truncated") {
		t.Error("truncated report should mention truncation")
	}
}

func TestReverseEngineerTool_Handle_ExistingArtifacts(t *testing.T) {
	root, cleanup := setupHandlerProject(t, setupGoProject)
	defer cleanup()

	// Create docs/ with a requirements file.
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	writeTestFile(t, root, "docs/requirements.md", "# Requirements\n\n- FR-001: Something\n")

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	text := getResultText(result)
	if !strings.Contains(text, "Existing SDD Artifacts") {
		t.Error("should have existing artifacts section")
	}
	if !strings.Contains(text, "requirements.md") {
		t.Error("should mention existing requirements.md")
	}
	if !strings.Contains(text, "missing") {
		t.Error("should mention missing artifacts")
	}
}

func TestReverseEngineerTool_Handle_AllArtifactsExist(t *testing.T) {
	root, cleanup := setupHandlerProject(t, setupGoProject)
	defer cleanup()

	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	writeTestFile(t, root, "docs/requirements.md", "# Requirements\n")
	writeTestFile(t, root, "docs/business-rules.md", "# Business Rules\n")
	writeTestFile(t, root, "docs/design.md", "# Design\n")

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, _ := tool.Handle(context.Background(), req)
	text := getResultText(result)

	if !strings.Contains(text, "All artifacts exist") {
		t.Error("should indicate all artifacts exist")
	}
}

func TestReverseEngineerTool_Handle_MaxDepth(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupGoProject)
	defer cleanup()

	tool := NewReverseEngineerTool()

	// Depth 0 should default to 3.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"max_depth": float64(0),
	}

	result, err := tool.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("expected success, got error: %s", getResultText(result))
	}
}

func TestReverseEngineerTool_Handle_TokenFooter(t *testing.T) {
	_, cleanup := setupHandlerProject(t, setupEmptyProject)
	defer cleanup()

	tool := NewReverseEngineerTool()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, _ := tool.Handle(context.Background(), req)
	text := getResultText(result)

	if !strings.Contains(text, "tokens") {
		t.Error("report should have token footer")
	}
}

// --- truncateReportToTokens unit tests ---

func TestTruncateReportToTokens_NoSections(t *testing.T) {
	report := "Just some text without any section headers"
	result := truncateReportToTokens(report, 5)

	if !strings.Contains(result, "truncated") {
		t.Error("should truncate simple text")
	}
}

func TestTruncateReportToTokens_WithSections(t *testing.T) {
	report := "# Title\n\nIntro paragraph\n\n## Section 1\n\nFirst content\n\n## Section 2\n\nSecond content\n\n## Section 3\n\nThird content lots and lots of content here"
	// Give enough budget for title + first section, but not all.
	result := truncateReportToTokens(report, 20)

	if !strings.Contains(result, "Title") {
		t.Error("should keep the title")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("should truncate later sections")
	}
}

func TestTruncateReportToTokens_FitsInBudget(t *testing.T) {
	report := "# Title\n\n## Section 1\n\nshort"
	result := truncateReportToTokens(report, 10000)

	if strings.Contains(result, "truncated") {
		t.Error("should not truncate when report fits in budget")
	}
	if result != report {
		t.Error("should return report unchanged when it fits")
	}
}

// --- Definition tests ---

func TestReverseEngineerTool_Definition(t *testing.T) {
	tool := NewReverseEngineerTool()
	def := tool.Definition()

	if def.Name != "sdd_reverse_engineer" {
		t.Errorf("tool name = %q, want %q", def.Name, "sdd_reverse_engineer")
	}
	if def.Description == "" {
		t.Error("definition should have a description")
	}
}
