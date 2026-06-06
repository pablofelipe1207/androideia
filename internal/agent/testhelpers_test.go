package agent

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/store"
)

// openTestStore crea un store en un directorio temporal.
func openTestStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	s, err := store.NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("error creating store: %v", err)
	}
	return s
}

// swapStdinReader reemplaza el reader que readUserResponse usa por r.
// Devuelve una función para restaurar el comportamiento por defecto.
func swapStdinReader(r io.Reader) func() {
	prev := stdinReader
	stdinReader = func() interface{ Read(p []byte) (n int, err error) } { return r }
	return func() { stdinReader = prev }
}

// noop assertion para mantener imports.
var _ = os.Stdin
