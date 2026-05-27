package handler

import (
	"encoding/base64"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// audioExtensions is the set of extensions always collected regardless of requested format.
var audioExtensions = map[string]bool{
	".flac": true,
	".mp3":  true,
	".wav":  true,
	".opus": true,
	".aac":  true,
}

// collectAudioFiles reads all audio files from dir, sorted alphabetically,
// and returns them as base64-encoded strings. Files matching audioFormat
// extension are included, as are any files with known audio extensions.
func collectAudioFiles(dir, audioFormat string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("collectAudioFiles: readdir %s: %v", dir, err)
		return nil
	}

	// Sort entries alphabetically (os.ReadDir already returns sorted on most platforms,
	// but we sort explicitly for correctness).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	targetExt := "." + strings.TrimPrefix(audioFormat, ".")
	var encoded []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != targetExt && !audioExtensions[ext] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			log.Printf("collectAudioFiles: read %s: %v", name, err)
			continue
		}
		encoded = append(encoded, base64.StdEncoding.EncodeToString(data))
	}

	if len(encoded) == 0 {
		log.Printf("collectAudioFiles: no audio files found in %s", dir)
	}
	return encoded
}
