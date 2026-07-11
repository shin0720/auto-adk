package setup

import (
	"os"
	"path/filepath"
	"strings"
)

func findDirsWithSuffix(dir, suffix string) []string {
	seen := make(map[string]bool)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			rel, _ := filepath.Rel(dir, filepath.Dir(path))
			if !seen[rel] {
				seen[rel] = true
			}
		}
		// Limit depth
		rel, _ := filepath.Rel(dir, path)
		if strings.Count(rel, string(filepath.Separator)) > 4 {
			return filepath.SkipDir
		}
		return nil
	})

	var dirs []string
	for d := range seen {
		dirs = append(dirs, d)
	}
	return dirs
}

func inferDirDescription(name string) string {
	descriptions := map[string]string{
		"cmd":        "CLI entry points",
		"pkg":        "Public reusable libraries",
		"internal":   "Private implementation packages",
		"api":        "API definitions and handlers",
		"web":        "Web server and routes",
		"src":        "Source code",
		"lib":        "Library code",
		"test":       "Test files",
		"tests":      "Test files",
		"docs":       "Documentation",
		"scripts":    "Build and utility scripts",
		"config":     "Configuration files",
		"templates":  "Template files",
		"assets":     "Static assets",
		"bin":        "Binary output",
		"build":      "Build output",
		"dist":       "Distribution output",
		"vendor":     "Vendored dependencies",
		"migrations": "Database migrations",
		"proto":      "Protocol buffer definitions",
		"content":    "Content assets",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return ""
}

func isIgnoredDir(name string) bool {
	ignored := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"__pycache__":  true,
		".git":         true,
		"dist":         true,
		"build":        true,
		"target":       true,
		".next":        true,
		".nuxt":        true,
		"coverage":     true,
		"Recent":       true, // Skip Windows Recent items
		"AppData":      true, // Skip AppData noise
	}
	return ignored[name]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
