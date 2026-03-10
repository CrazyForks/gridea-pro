package main

import (
	"gridea-pro/backend/internal/mcp"
	"log/slog"
	"os"
)

func main() {
	if _, err := os.Stat(mcp.GetAppDir()); os.IsNotExist(err) {
		slog.Error("SOURCE_DIR not found", "path", mcp.GetAppDir())
		slog.Error("Please set SOURCE_DIR environment variable to your Gridea Pro data directory.")
		os.Exit(1)
	}

	server := mcp.NewServer()
	if err := server.Start(); err != nil {
		slog.Error("Error starting MCP server", "error", err)
		os.Exit(1)
	}
}
