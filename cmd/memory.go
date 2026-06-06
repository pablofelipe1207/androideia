package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/memory"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Gestiona las conversaciones persistidas del agente",
	Long:  `Lista, muestra y elimina sesiones guardadas del agente en .androideai/core.db.`,
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista las conversaciones guardadas",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		convs, err := loadConversations(limit)
		if err != nil {
			return err
		}
		if len(convs) == 0 {
			fmt.Println("No hay conversaciones guardadas.")
			return nil
		}

		fmt.Printf("Encontradas %d conversaciones:\n\n", len(convs))
		fmt.Println("ID      STATUS       UPDATED              TÍTULO / TAREA")
		fmt.Println(strings.Repeat("─", 90))
		for _, c := range convs {
			title := c.Title
			if len(title) > 50 {
				title = title[:50] + "…"
			}
			ts := time.Unix(c.UpdatedAt, 0).Format("2006-01-02 15:04")
			fmt.Printf("%-7d %-12s %-20s %s\n", c.ID, c.Status, ts, title)
		}
		fmt.Println("\nPara ver mensajes:  androideai memory show <id>")
		fmt.Println("Para continuar:     androideai agent --resume <id> \"<nuevo mensaje>\"")
		return nil
	},
}

var memoryShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Muestra los mensajes de una conversación",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("ID inválido: %s", args[0])
		}
		conv, msgs, err := loadConversationMessages(id)
		if err != nil {
			return err
		}

		fmt.Printf("Conversación #%d\n", conv.ID)
		fmt.Printf("  Título:   %s\n", conv.Title)
		fmt.Printf("  Estado:   %s\n", conv.Status)
		fmt.Printf("  Tarea:    %s\n", conv.Task)
		if conv.Provider != "" {
			fmt.Printf("  Provider: %s / %s\n", conv.Provider, conv.Model)
		}
		fmt.Printf("  Creada:   %s\n", time.Unix(conv.CreatedAt, 0).Format("2006-01-02 15:04:05"))
		fmt.Printf("  Updated:  %s\n", time.Unix(conv.UpdatedAt, 0).Format("2006-01-02 15:04:05"))
		fmt.Println()
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println("MENSAJES")
		fmt.Println(strings.Repeat("═", 80))

		for i, m := range msgs {
			header := fmt.Sprintf("[%d] %s", i+1, strings.ToUpper(m.Role))
			if m.ToolName != "" {
				header += " (" + m.ToolName + ")"
			}
			fmt.Println()
			fmt.Println(header)
			fmt.Println(strings.Repeat("─", 80))
			if m.Content != "" {
				fmt.Println(m.Content)
			}
			if len(m.ToolCalls) > 0 {
				fmt.Println("[tool_calls]:")
				b, _ := json.MarshalIndent(m.ToolCalls, "  ", "  ")
				fmt.Println("  " + string(b))
			}
		}
		return nil
	},
}

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Elimina una conversación y sus mensajes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("ID inválido: %s", args[0])
		}
		mem, closer, err := openMemory()
		if err != nil {
			return err
		}
		defer closer()

		if err := mem.DeleteConversation(id); err != nil {
			return err
		}
		fmt.Printf("Conversación %d eliminada.\n", id)
		return nil
	},
}

var memoryPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Elimina TODAS las conversaciones",
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Print("¿Eliminar TODAS las conversaciones? [y/N]: ")
			response := readUserResponse()
			if strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes" {
				fmt.Println("Cancelado.")
				return nil
			}
		}
		mem, closer, err := openMemory()
		if err != nil {
			return err
		}
		defer closer()

		convs, err := mem.ListConversations(0)
		if err != nil {
			return err
		}
		count := 0
		for _, c := range convs {
			if err := mem.DeleteConversation(c.ID); err == nil {
				count++
			}
		}
		fmt.Printf("Eliminadas %d conversaciones.\n", count)
		return nil
	},
}

// readUserResponse lee una línea de stdin. Si no hay nada (tests/EOF), devuelve "".
func readUserResponse() string {
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return strings.TrimRight(sb.String(), "\r")
			}
			sb.WriteByte(buf[0])
		}
		if err != nil {
			return strings.TrimRight(sb.String(), "\r")
		}
		if buf[0] == '\n' {
			break
		}
	}
	return strings.TrimRight(sb.String(), "\r")
}

func openMemory() (*memory.Memory, func(), error) {
	dbPath := filepath.Join(".androideai", "core.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("database not found, run 'androideai init' first")
	}
	s, err := store.NewStore(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error opening database: %w", err)
	}
	return memory.NewMemory(s.DB()), func() { _ = s.Close() }, nil
}

func loadConversations(limit int) ([]*memory.Conversation, error) {
	mem, closer, err := openMemory()
	if err != nil {
		return nil, err
	}
	defer closer()
	return mem.ListConversations(limit)
}

func loadConversationMessages(id int64) (*memory.Conversation, []memory.StoredMessage, error) {
	mem, closer, err := openMemory()
	if err != nil {
		return nil, nil, err
	}
	defer closer()

	conv, err := mem.GetConversation(id)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := mem.LoadMessages(id)
	if err != nil {
		return nil, nil, err
	}
	return conv, msgs, nil
}

func init() {
	memoryListCmd.Flags().IntP("limit", "n", 20, "Número máximo de conversaciones a mostrar")
	memoryPurgeCmd.Flags().BoolP("yes", "y", false, "No pedir confirmación")

	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memoryShowCmd)
	memoryCmd.AddCommand(memoryDeleteCmd)
	memoryCmd.AddCommand(memoryPurgeCmd)
}
