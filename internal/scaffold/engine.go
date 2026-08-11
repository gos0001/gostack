package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	// tmplSuffix marks a template rendered with {{...}} at generation time.
	tmplSuffix = ".tmpl"

	// rawTmplSuffix marks a template whose *output* contains {{...}} that must
	// survive generation — Markdown documenting Go templates, wire sets and gin
	// routes. Substitution in these files uses <<...>> instead.
	// "SKILL.md.raw.tmpl" → "SKILL.md".
	rawTmplSuffix = ".raw.tmpl"
)

// delims selects the action delimiters a template is parsed with.
type delims struct {
	left, right string
	// emptyIsError makes a template that renders to nothing a hard failure
	// rather than a silent no-op.
	emptyIsError bool
}

var (
	// goDelims is the default: {{...}} is evaluated at generation time.
	goDelims = delims{left: "{{", right: "}}"}

	// altDelims is for files whose output is itself Go template source — HTML
	// views, and the Markdown that documents them. Parsing such a file with
	// goDelims is not merely wrong, it is silently wrong: a file of {{define}}
	// blocks executes to the empty string, because executing a template emits
	// only the text outside its definitions. These files are almost entirely
	// literal text, so an empty render can only be a mistake — hence the error.
	altDelims = delims{left: "<<", right: ">>", emptyIsError: true}
)

// Engine renders embedded template trees into a destination directory.
type Engine struct {
	fs embed.FS
}

// NewEngine creates an engine backed by the embedded FS.
func NewEngine(efs embed.FS) *Engine {
	return &Engine{fs: efs}
}

// RenderLayers renders each layer in order into the same destination directory.
// A missing layer is skipped, so optional layers need not exist. Layers listed
// later win on filename collisions — but no two layers in ProjectLayers
// contribute the same *file*, so a collision means a template was filed under
// the wrong layer. Several layers may contribute to the same directory: the
// skill's sections/ is filled from base plus each optional layer.
func (e *Engine) RenderLayers(destDir string, ctx TemplateContext, layers ...string) error {
	for _, layer := range layers {
		if _, err := fs.Stat(e.fs, layer); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat layer %s: %w", layer, err)
		}
		if err := e.RenderTree(layer, destDir, ctx); err != nil {
			return fmt.Errorf("render layer %s: %w", layer, err)
		}
	}
	return nil
}

// ProjectLayers returns the embedded template roots selected by ctx's features.
func ProjectLayers(ctx TemplateContext) []string {
	layers := []string{"project/base"}
	if ctx.WithFrontend {
		layers = append(layers, "project/opt_frontend")
	}
	if ctx.WithPostgres {
		layers = append(layers, "project/opt_postgres")
	}
	if ctx.WithRedis {
		layers = append(layers, "project/opt_redis")
	}
	if ctx.WithDocker {
		layers = append(layers, "project/opt_docker")
	}
	return layers
}

// RenderTree walks srcDir inside the embedded FS, renders every .tmpl file
// through text/template with ctx, and copies non-.tmpl files verbatim.
// Destination paths are written under destDir with .tmpl suffix stripped.
func (e *Engine) RenderTree(srcDir, destDir string, ctx TemplateContext) error {
	return fs.WalkDir(e.fs, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// embed.FS paths are always slash-separated; trim the prefix by hand
		// rather than using filepath.Rel, whose separator handling differs on
		// Windows.
		rel := strings.TrimPrefix(strings.TrimPrefix(path, srcDir), "/")
		if rel == "" {
			return os.MkdirAll(destDir, 0755)
		}
		// Expand template variables in the path itself.
		rel = expandPath(rel, ctx)
		dest := filepath.Join(destDir, filepath.FromSlash(rel))

		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}

		data, err := e.fs.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		// Order matters: ".raw.tmpl" also ends in ".tmpl", so it must be tested
		// first or every doc would be parsed with the wrong delimiters.
		switch {
		case strings.HasSuffix(dest, rawTmplSuffix):
			return renderWith(altDelims, strings.TrimSuffix(dest, rawTmplSuffix), string(data), ctx)

		case strings.HasSuffix(dest, tmplSuffix):
			return renderWith(goDelims, strings.TrimSuffix(dest, tmplSuffix), string(data), ctx)

		default:
			// Non-template files are copied verbatim (JS, CSS, HTML views).
			return writeFile(dest, data)
		}
	})
}

// RenderFile renders a single embedded .tmpl file to a specific dest path.
func (e *Engine) RenderFile(tmplPath, destPath string, ctx TemplateContext) error {
	data, err := e.fs.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplPath, err)
	}
	return renderTemplate(destPath, string(data), ctx)
}

// RenderAltDelims renders a single embedded template whose output is itself Go
// template source — an HTML view, or Markdown documenting one.
//
// Substitution uses <<...>> so that {{...}} passes through untouched. Rendering
// such a file with the default delimiters silently produces an empty result:
// {{define}} blocks are definitions, and executing the template emits only the
// text outside them — which is nothing. That silent failure is exactly why
// altDelims treats an empty render as an error.
func (e *Engine) RenderAltDelims(tmplPath, destPath string, ctx TemplateContext) error {
	data, err := e.fs.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplPath, err)
	}
	return renderWith(altDelims, destPath, string(data), ctx)
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"toLower":  strings.ToLower,
		"toUpper":  strings.ToUpper,
		"toPascal": toPascal,
		"toSnake":  toSnake,
		"join":     strings.Join,
	}
}

// renderTemplate parses content with the default {{...}} delimiters.
func renderTemplate(dest, content string, ctx TemplateContext) error {
	return renderWith(goDelims, dest, content, ctx)
}

// renderWith parses content with d's delimiters, executes it with ctx, and
// writes the result to dest.
func renderWith(d delims, dest, content string, ctx TemplateContext) error {
	t, err := template.New(filepath.Base(dest)).
		Delims(d.left, d.right).
		Funcs(templateFuncs()).
		Parse(content)
	if err != nil {
		return fmt.Errorf("parse template for %s: %w", dest, err)
	}

	// Render into a buffer first: os.Create truncates, so writing directly
	// would leave an empty file behind if Execute failed.
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("execute template for %s: %w", dest, err)
	}

	if strings.TrimSpace(buf.String()) == "" {
		if d.emptyIsError {
			return fmt.Errorf(
				"template for %s rendered empty — wrong delimiters, or a conditional swallowed the whole file", dest)
		}
		// A {{...}} template that renders to nothing opts out of producing a
		// file. This lets a whole file be switched off with an {{if}} wrapper,
		// without needing its own layer.
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, buf.Bytes(), 0644)
}

// writeFile writes raw bytes to dest, creating parent dirs as needed.
func writeFile(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

// expandPath substitutes template variables inside a file path segment.
// Supports: __PackageName__, __EntityName__, __EntityPlural__, __PascalEntity__
func expandPath(path string, ctx TemplateContext) string {
	r := strings.NewReplacer(
		"__ModulePath__", sanitizePath(ctx.ModulePath),
		"__ProjectName__", ctx.ProjectName,
		"__PackageName__", ctx.PackageName,
		"__FeatureName__", ctx.FeatureName,
		"__PascalName__", ctx.PascalName,
		"__EntityName__", ctx.EntityName,
		"__EntityPlural__", ctx.EntityPlural,
		"__PascalEntity__", ctx.PascalEntity,
	)
	return r.Replace(path)
}

func sanitizePath(p string) string {
	return strings.ReplaceAll(p, "/", "_")
}
