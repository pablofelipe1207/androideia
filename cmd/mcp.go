package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pablofelipe1207/androideia/internal/mcpclient"
	"github.com/pablofelipe1207/androideia/internal/mcpserver"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Comandos MCP (Model Context Protocol)",
	Long:  `Gestiona conexiones MCP y exponer herramientas del proyecto a otros agentes (OpenCode, Claude Code, etc.).`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Inicia el servidor MCP",
	Long: `Inicia un servidor MCP que expone las herramientas de androideia a otros agentes.
El servidor se comunica por stdio (stdin/stdout) siguiendo el protocolo MCP.

Uso con OpenCode (opencode.json):
{
  "mcp": {
    "servers": {
      "androideia": {
        "command": "androideai",
        "args": ["mcp", "serve"],
        "cwd": "/ruta/a/tu/proyecto"
      }
    }
  }
}

Uso con Claude Desktop (claude_desktop_config.json):
{
  "mcpServers": {
    "androideia": {
      "command": "androideai",
      "args": ["mcp", "serve"],
      "cwd": "/ruta/a/tu/proyecto"
    }
  }
}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("base de datos no encontrada en %s. Ejecutá 'androideai init' primero", dbPath)
		}

		srv, err := mcpserver.New(dbPath)
		if err != nil {
			return fmt.Errorf("error iniciando servidor MCP: %w", err)
		}
		defer srv.Close()

		// Handle signals for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			fmt.Fprintln(os.Stderr, "\nShutting down MCP server...")
			srv.Close()
			os.Exit(0)
		}()

		fmt.Fprintln(os.Stderr, "androideia MCP server ready")
		if err := srv.Run(cmd.Context()); err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}

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

		fmt.Printf("Conectando a servidor MCP: %s\n", serverURL)

		client := mcpclient.NewMCPClient(serverURL)

		ctx := cmd.Context()
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("error conectando al servidor MCP: %w", err)
		}
		defer client.Disconnect(ctx)

		fmt.Println("Conexión exitosa!")

		tools, err := client.ListTools(ctx)
		if err != nil {
			return fmt.Errorf("error listando tools: %w", err)
		}

		fmt.Printf("\nTools disponibles (%d):\n", len(tools))
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
		fmt.Println("Configuración MCP para conectar agentes:")
		fmt.Println()
		fmt.Println("1. Copiá una de las siguientes configuraciones en tu agente:")
		fmt.Println()
		fmt.Println("   Para OpenCode (opencode.json):")
		fmt.Println("   {")
		fmt.Println(`     "mcp": {`)
		fmt.Println(`       "servers": {`)
		fmt.Println(`         "androideia": {`)
		fmt.Println(`           "command": "androideai",`)
		fmt.Println(`           "args": ["mcp", "serve"]`)
		fmt.Println(`         }`)
		fmt.Println(`       }`)
		fmt.Println(`     }`)
		fmt.Println("   }")
		fmt.Println()
		fmt.Println("   Para Claude Desktop (claude_desktop_config.json):")
		fmt.Println("   {")
		fmt.Println(`     "mcpServers": {`)
		fmt.Println(`       "androideia": {`)
		fmt.Println(`         "command": "androideai",`)
		fmt.Println(`         "args": ["mcp", "serve"]`)
		fmt.Println(`       }`)
		fmt.Println(`     }`)
		fmt.Println("   }")
		fmt.Println()
		fmt.Println("2. Tools disponibles:")
		fmt.Println("   Semántica: semantic_search, semantic_locate, semantic_graph, semantic_deps, semantic_suggest, semantic_index")
		fmt.Println("   Cerebro:   brain_search, brain_save, brain_list, brain_review, brain_promote")
		fmt.Println("   Plantillas: scaffold_template, scaffold_list, scaffold_validate")
		fmt.Println("   Tareas:    task_list, task_create, task_get, task_stats")
		fmt.Println("   Proyecto:  project_info")

		return nil
	},
}

func init() {
	projectDir, _ := os.Getwd()
	defaultDB := filepath.Join(projectDir, ".androideai", "core.db")

	mcpServeCmd.Flags().String("db", defaultDB, "Ruta a la base de datos SQLite")

	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpConnectCmd)
	mcpCmd.AddCommand(mcpListCmd)
}
