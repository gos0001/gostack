package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppendToWireBuild adds `pkgAlias.Set,` to the wire.Build(...) call in cmd/wire.go
// and adds the corresponding import.
func AppendToWireBuild(projectDir, modulePath, pkgPath, pkgAlias string) error {
	wirePath := filepath.Join(projectDir, "cmd", "wire.go")

	data, err := os.ReadFile(wirePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", wirePath, err)
	}

	original := string(data)
	content := original

	// Add import if not already present.
	importLine := fmt.Sprintf("\t%s %q", pkgAlias, modulePath+"/"+pkgPath)
	if !strings.Contains(content, importLine) {
		content = insertImport(content, importLine)
	}

	// Insert Set before the closing ) of wire.Build(
	setLine := fmt.Sprintf("\t\t%s.Set,", pkgAlias)
	if !strings.Contains(content, setLine) {
		content = insertBeforeWireBuildClose(content, setLine)
	}

	// Write only when something actually changed — but never skip a write just
	// because the Set line was already there, or a freshly added import is lost.
	if content == original {
		return nil
	}

	return os.WriteFile(wirePath, []byte(content), 0644)
}

// Splice markers. Generated files carry these comments so insertions have a
// stable anchor that does not depend on the surrounding code's shape.
const (
	MarkerImports   = "// gostack:imports"
	MarkerParams    = "// gostack:params"
	MarkerRoutes    = "// gostack:routes"
	MarkerProviders = "// gostack:providers"
)

// insertBeforeMarker inserts line immediately above the line containing marker.
// ok is false when the marker is absent, so callers can fall back to the older
// heuristics for projects generated before markers existed.
func insertBeforeMarker(content, marker, line string) (string, bool) {
	idx := strings.Index(content, marker)
	if idx < 0 {
		return content, false
	}
	lineStart := strings.LastIndex(content[:idx], "\n") + 1
	return content[:lineStart] + line + "\n" + content[lineStart:], true
}

// insertImport adds a line into the import block, before its closing ")".
func insertImport(content, line string) string {
	if out, ok := insertBeforeMarker(content, MarkerImports, line); ok {
		return out
	}
	importEnd := strings.Index(content, "\n)")
	if importEnd < 0 {
		return content
	}
	// Splice before the newline that precedes ")", so the new import keeps its
	// own line. Including that newline here would jam the line against ")" and
	// destroy the "\n)" anchor for every subsequent insert.
	return content[:importEnd] + "\n" + line + content[importEnd:]
}

// insertBeforeWireBuildClose inserts a line before the closing ")" of wire.Build(
func insertBeforeWireBuildClose(content, line string) string {
	if out, ok := insertBeforeMarker(content, MarkerProviders, line); ok {
		return out
	}
	// Find wire.Build( and then its closing )
	buildIdx := strings.Index(content, "wire.Build(")
	if buildIdx < 0 {
		return content
	}
	// Find closing paren after wire.Build(
	depth := 0
	closeIdx := -1
	for i := buildIdx; i < len(content); i++ {
		switch content[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closeIdx = i
				break
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return content
	}
	return content[:closeIdx] + line + "\n\t" + content[closeIdx:]
}
