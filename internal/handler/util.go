package handler

import (
	"fmt"
	"io"
	"math/rand"
	"os"
)

// newRequestID returns a short random hex ID for correlating bridge requests.
func newRequestID() string {
	return fmt.Sprintf("%016x", rand.Uint64())
}

// truncate shortens s to at most n characters, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// saveUpload copies an uploaded file to destPath.
func saveUpload(src io.Reader, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("saveUpload: create %s: %w", destPath, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, src); err != nil {
		return fmt.Errorf("saveUpload: copy: %w", err)
	}
	return nil
}

// makeTempDir creates a temp directory, registers it with the handler's registry
// (if set), and returns the path and a cleanup function that removes it and
// deregisters it.
func (h *Handlers) makeTempDir(prefix string) (string, func(), error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", nil, err
	}
	if h.Registry != nil {
		h.Registry.Add(dir)
	}
	cleanup := func() {
		os.RemoveAll(dir)
		if h.Registry != nil {
			h.Registry.Remove(dir)
		}
	}
	return dir, cleanup, nil
}
