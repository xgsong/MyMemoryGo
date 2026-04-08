// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xgsong/MyMemoryGo/internal/interface/api"
)

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the REST API server",
	Long: `Start the REST API server for the memory service.

The server provides REST endpoints for:
- POST /api/v1/memories - Store a memory
- GET /api/v1/memories/:id - Get a memory by ID
- GET /api/v1/memories - List memories
- DELETE /api/v1/memories/:id - Delete a memory
- POST /api/v1/search - Search memories
- POST /api/v1/sync - Sync file index
- GET /health - Health check
- GET /ready - Readiness check`,
	RunE: runServe,
}

var (
	serveHost string
	servePort int
)

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveHost, "host", "", "server host (default from config)")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 0, "server port (default from config)")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize app
	app, err := InitializeApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	defer app.Cleanup()

	// Get server configuration
	host := serveHost
	if host == "" {
		host = viper.GetString("api.host")
		if host == "" {
			host = "0.0.0.0"
		}
	}

	port := servePort
	if port == 0 {
		port = viper.GetInt("api.port")
		if port == 0 {
			port = 8080
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// Create API server
	server := api.NewServer(app.MemoryApp)

	fmt.Printf("Starting REST API server on %s\n", addr)
	fmt.Println()
	fmt.Println("Endpoints:")
	fmt.Println("  POST   /api/v1/memories     - Store a memory")
	fmt.Println("  GET    /api/v1/memories/:id - Get a memory")
	fmt.Println("  GET    /api/v1/memories     - List memories")
	fmt.Println("  DELETE /api/v1/memories/:id - Delete a memory")
	fmt.Println("  POST   /api/v1/search       - Search memories")
	fmt.Println("  POST   /api/v1/sync         - Sync index")
	fmt.Println("  GET    /health              - Health check")
	fmt.Println("  GET    /ready               - Readiness check")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")

	// Wait for interrupt
	ctx = WaitForInterrupt(ctx)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(addr); err != nil {
			errChan <- err
		}
	}()

	// Wait for interrupt or server error
	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down server...")
		shutdownCtx := context.Background()
		server.Shutdown(shutdownCtx)
		fmt.Println("Server stopped.")
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
