// SDD-Hoffy: Spec-Driven Development MCP Server
//
// A universal MCP server that integrates with any AI coding tool
// (Claude Code, OpenCode, Gemini CLI, Codex, Cursor, VS Code Copilot)
// to guide users from vague ideas to clear, actionable specifications.
//
// Usage:
//
//	sdd-hoffy serve    # Start MCP server (stdio transport)
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	sddserver "github.com/HendryAvila/sdd-hoffy/internal/server"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if err := run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "--help", "-h", "help":
		printUsage()
		os.Exit(0)
	case "--version", "-v", "version":
		fmt.Printf("sdd-hoffy v%s\n", sddserver.Version)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func run() error {
	s, err := sddserver.New()
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	// Graceful shutdown on interrupt.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	_ = ctx // stdio server manages its own lifecycle

	return server.ServeStdio(s)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `SDD-Hoffy v%s — Spec-Driven Development MCP Server

Usage:
  sdd-hoffy serve    Start the MCP server (stdio transport)

Configuration:
  Add to your AI tool's MCP config:

  {
    "mcpServers": {
      "sdd-hoffy": {
        "command": "sdd-hoffy",
        "args": ["serve"]
      }
    }
  }

Learn more: https://github.com/HendryAvila/sdd-hoffy
`, sddserver.Version)
}
