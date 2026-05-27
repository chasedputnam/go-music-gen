package tmpdir

import (
	"log"
	"os"
	"sync"
)

// Registry tracks active temporary directories so they can be cleaned up
// on server shutdown even if a request handler didn't clean up after itself.
type Registry struct {
	mu   sync.Mutex
	dirs map[string]struct{}
}

// NewRegistry creates a new Registry.
func NewRegistry() *Registry {
	return &Registry{dirs: make(map[string]struct{})}
}

// Add registers a temp directory.
func (r *Registry) Add(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirs[dir] = struct{}{}
}

// Remove unregisters a temp directory (call after successful cleanup).
func (r *Registry) Remove(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.dirs, dir)
}

// Cleanup removes all remaining registered directories. Call on shutdown.
func (r *Registry) Cleanup() {
	r.mu.Lock()
	dirs := make([]string, 0, len(r.dirs))
	for d := range r.dirs {
		dirs = append(dirs, d)
	}
	r.mu.Unlock()

	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			log.Printf("tmpdir: cleanup %s: %v", d, err)
		} else {
			log.Printf("tmpdir: cleaned up %s", d)
		}
	}
}
