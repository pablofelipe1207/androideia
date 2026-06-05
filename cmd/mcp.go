package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mobiai/androideai-core/internal/mcpclient"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Comandos MCP (Model Context Protocol)",
	Long:  `Gestiona conexiones MCP y exponer herramientas del producto.`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Inicia el servidor MCP",
	Long:  `Inicia un servidor MCP que expone las herramientas de androideai-core a otros agentes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		
		fmt.Printf("Starting MCP server on port %d...\n", port)
		fmt.Println("Note: MCP server implementation is a stub in this phase.")
		fmt.Println("Full MCP server will be implemented in future versions.")
		
		// Create context with signal handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		
		go func() {
			<-sigCh
			fmt.Println("\nShutting down MCP server...")
			cancel()
		}()

		// Server would run here
		fmt.Println("MCP server ready. Press Ctrl+C to stop.")
		
		// Wait for context cancellation
		<-ctx.Done()
		
		fmt.Println("MCP server stopped.")
		return nil
	},
}

var mcpConnectCmd = &cobra.Command{
	Use:   "connect [url]",
	Short: "Conecta a un servidor MCP externo",
	Long:  `Conecta a un servidor MCP externo y lista sus herramientas disponibles.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := args[0]
		
		fmt.Printf("Connecting to MCP server: %s\n", serverURL)
		
		// Create client
		client := mcpclient.NewMCPClient(serverURL)
		
		// Connect
		ctx, cancel := context.WithTimeout(context.Background(), 30)
		defer cancel()
		
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("error connecting to MCP server: %w", err)
		}
		defer client.Disconnect(ctx)
		
		fmt.Println("Connected successfully!")
		
		// List tools
		tools, err := client.ListTools(ctx)
		if err != nil {
			return fmt.Errorf("error listing tools: %w", err)
		}
		
		fmt.Printf("\nAvailable tools (%d):\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
		}
		
		return nil
	},
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista servidores MCP configurados",
	Long:  `Muestra la lista de servidores MCP configurados en la configuración del proyecto.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Configured MCP servers:")
		fmt.Println("(No servers configured yet)")
		fmt.Println("\nTo configure MCP servers, add them to your config.yml:")
		fmt.Println("mcp:")
		fmt.Println("  servers:")
		fmt.Println("    - name: my-server")
		fmt.Println("      url: http://localhost:3000")
		return nil
	},
}

func init() {
	// Serve command flags
	mcpServeCmd.Flags().IntP("port", "p", 3000, "Port to listen on")
	
	// Add commands
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpConnectCmd)
	mcpCmd.AddCommand(mcpListCmd)
}
