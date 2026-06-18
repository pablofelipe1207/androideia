package mcpserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/scaffold"
	"github.com/pablofelipe1207/androideia/internal/semantic"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/pablofelipe1207/androideia/internal/task"
	"github.com/pablofelipe1207/androideia/internal/version"
)

type Server struct {
	mcp     *mcp.Server
	db      *sql.DB
	store   *store.Store
	brain   *brain.Brain
	semantic *semantic.Semantic
	tasks   *task.TaskManager
}

func New(dbPath string) (*Server, error) {
	s, err := store.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening store: %w", err)
	}

	b := brain.NewBrain(s.DB())
	tm := task.NewTaskManager(s.DB())

	srv := &Server{
		store:   s,
		db:      s.DB(),
		brain:   b,
		tasks:   tm,
		semantic: semantic.NewSemantic(s.DB(), "", ""),
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "androideia",
		Version: version.Version,
	}, &mcp.ServerOptions{
		Instructions: "androideia MCP server - Agente de desarrollo Android offline-first. " +
			"Proporciona acceso a la semántica del proyecto, memoria del cerebro, " +
			"plantillas canónicas Android y gestión de tareas.",
		Logger: slog.Default(),
	})

	srv.mcp = mcpServer
	srv.registerTools()

	return srv, nil
}

func (s *Server) registerTools() {
	registerSemanticTools(s)
	registerBrainTools(s)
	registerScaffoldTools(s)
	registerTaskTools(s)
	registerProjectTools(s)
}

func (s *Server) Run(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) Close() error {
	return s.store.Close()
}

func (s *Server) MCP() *mcp.Server {
	return s.mcp
}

func scaffoldAllRoles() []string {
	roles := scaffold.AllRoles()
	out := make([]string, len(roles))
	for i, r := range roles {
		out[i] = string(r)
	}
	return out
}
