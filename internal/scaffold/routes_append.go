package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// AppendPageRoute adds a new page handler to the generated SSR router at
// WebDir/routes.go. It updates the import block, the New() parameters, and the
// route registration body.
func AppendPageRoute(projectDir, modulePath string, ctx TemplateContext) error {
	routesPath := filepath.Join(projectDir, filepath.FromSlash(WebDir), "routes.go")

	data, err := os.ReadFile(routesPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", routesPath, err)
	}

	content := string(data)
	pkgPath := PagePkgPath(ctx.FsPagePath) // fs-safe path, slash-separated
	pkgAlias := PageAlias(ctx.FsPagePath)
	handlerType := fmt.Sprintf("*%s.Handler", pkgAlias)
	paramName := pkgAlias

	// 1. Add import
	importLine := fmt.Sprintf("\t%s %q", pkgAlias, modulePath+"/"+pkgPath)
	if !strings.Contains(content, importLine) {
		content = insertImport(content, importLine)
	}

	// 2. Add parameter to New() signature
	paramLine := fmt.Sprintf("\t%s %s,", paramName, handlerType)
	if !strings.Contains(content, paramLine) {
		content = insertParam(content, paramLine)
	}

	// 3. Add route registration
	routeLine := routeRegistration(ctx, paramName)
	if !strings.Contains(content, routeLine) {
		content = insertRoute(content, routeLine)
	}

	return os.WriteFile(routesPath, []byte(content), 0644)
}

// AppendAPIRoute registers a single JSON API handler in the http_v1 controller:
// import, constructor parameter, and route line.
func AppendAPIRoute(projectDir, modulePath string, ctx TemplateContext, method, route string) error {
	controllerPath := filepath.Join(projectDir, "internal", "controller", "http_v1", "controller.go")

	data, err := os.ReadFile(controllerPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", controllerPath, err)
	}

	original := string(data)
	content := original
	alias := ctx.PackageName

	importLine := fmt.Sprintf("\t%s %q", alias, modulePath+"/"+UsecasePkgPath(ctx))
	if !strings.Contains(content, importLine) {
		content = insertImport(content, importLine)
	}

	paramLine := fmt.Sprintf("\t%sH *%s.HTTPv1Handler,", alias, alias)
	if !strings.Contains(content, paramLine) {
		content = insertParam(content, paramLine)
	}

	routeLine := fmt.Sprintf("\tv1.%s(%q, %sH.Handle)", method, route, alias)
	if !strings.Contains(content, routeLine) {
		content = insertRoute(content, routeLine)
	}

	if content == original {
		return nil
	}
	return os.WriteFile(controllerPath, []byte(content), 0644)
}

// orchestratorSpec describes how a use case registers itself with one
// orchestrator: which file to splice into, which handler type the use case
// package must expose, and the line that appends it to the orchestrator's
// slice. Adding a third orchestrator is one entry here plus its templates.
type orchestratorSpec struct {
	pkg         string // directory and package under internal/orchestrator
	handlerType string // handler type exported by the use case package
	paramSuffix string // keeps the parameter name distinct from the package alias
	appendFmt   string // registration line; the verb takes the parameter name
}

var orchestratorSpecs = map[string]orchestratorSpec{
	"bootstrap": {
		pkg:         "bootstrap",
		handlerType: "BootstrapHandler",
		paramSuffix: "B",
		appendFmt:   "\tb.tasks = append(b.tasks, %s)",
	},
	"workers": {
		pkg:         "workers",
		handlerType: "WorkersHandler",
		paramSuffix: "W",
		appendFmt:   "\tw.workers = append(w.workers, %s)",
	},
}

// OrchestratorNames returns the orchestrators a use case can register with,
// sorted so help text and error messages are stable across runs.
func OrchestratorNames() []string {
	names := make([]string, 0, len(orchestratorSpecs))
	for name := range orchestratorSpecs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidateOrchestrator rejects an unknown --orchestrator value.
func ValidateOrchestrator(name string) error {
	if _, ok := orchestratorSpecs[name]; !ok {
		return fmt.Errorf("unknown orchestrator %q — valid values: %s",
			name, strings.Join(OrchestratorNames(), ", "))
	}
	return nil
}

// AppendOrchestratorTask registers a use case's handler with an orchestrator:
// import, constructor parameter, and the append line in the registration body.
// The same three edits AppendAPIRoute makes to the HTTP controller — an
// orchestrator is a caller like any other, it just is not a network one.
func AppendOrchestratorTask(projectDir, modulePath, orchestrator string, ctx TemplateContext) error {
	spec, ok := orchestratorSpecs[orchestrator]
	if !ok {
		return fmt.Errorf("unknown orchestrator %q", orchestrator)
	}

	filePath := filepath.Join(projectDir, "internal", "orchestrator", spec.pkg, spec.pkg+".go")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	original := string(data)
	content := original
	alias := ctx.PackageName
	param := alias + spec.paramSuffix

	importLine := fmt.Sprintf("\t%s %q", alias, modulePath+"/"+UsecasePkgPath(ctx))
	if !strings.Contains(content, importLine) {
		content = insertImport(content, importLine)
	}

	paramLine := fmt.Sprintf("\t%s *%s.%s,", param, alias, spec.handlerType)
	if !strings.Contains(content, paramLine) {
		content = insertParam(content, paramLine)
	}

	appendLine := fmt.Sprintf(spec.appendFmt, param)
	if !strings.Contains(content, appendLine) {
		content = insertRoute(content, appendLine)
	}

	if content == original {
		return nil
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}

// AppendCRUDRoutes adds CRUD routes to internal/controller/http_v1/controller.go.
func AppendCRUDRoutes(projectDir, modulePath string, ctx TemplateContext) error {
	controllerPath := filepath.Join(projectDir, "internal", "controller", "http_v1", "controller.go")

	data, err := os.ReadFile(controllerPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", controllerPath, err)
	}

	content := string(data)

	// Add 5 handler imports and params
	ops := []struct{ name, method, path string }{
		{ctx.EntityName + "_get", "GET", "/:id"},
		{ctx.EntityName + "_list", "GET", ""},
		{ctx.EntityName + "_create", "POST", ""},
		{ctx.EntityName + "_update", "PUT", "/:id"},
		{ctx.EntityName + "_delete", "DELETE", "/:id"},
	}

	for _, op := range ops {
		alias := strings.ReplaceAll(op.name, "-", "_")
		// UsecasePkgPathFor handles both grouped and legacy flat layouts:
		// an empty GroupPath yields the old internal/usecases/<pkg> path.
		pkgPath := UsecasePkgPathFor(ctx.GroupPath, op.name)
		importLine := fmt.Sprintf("\t%s %q", alias, modulePath+"/"+pkgPath)
		if !strings.Contains(content, importLine) {
			content = insertImport(content, importLine)
		}
		paramLine := fmt.Sprintf("\t%sH *%s.HTTPv1Handler,", alias, alias)
		if !strings.Contains(content, paramLine) {
			content = insertParam(content, paramLine)
		}
	}

	// Add route group
	groupLines := fmt.Sprintf(
		"\n\t%s := v1.Group(%q)\n"+
			"\t%s.GET(\"/:id\", %s_getH.Handle)\n"+
			"\t%s.GET(\"\", %s_listH.Handle)\n"+
			"\t%s.POST(\"\", %s_createH.Handle)\n"+
			"\t%s.PUT(\"/:id\", %s_updateH.Handle)\n"+
			"\t%s.DELETE(\"/:id\", %s_deleteH.Handle)\n",
		ctx.EntityPlural+"Group", "/"+ctx.EntityPlural,
		ctx.EntityPlural+"Group", ctx.EntityName,
		ctx.EntityPlural+"Group", ctx.EntityName,
		ctx.EntityPlural+"Group", ctx.EntityName,
		ctx.EntityPlural+"Group", ctx.EntityName,
		ctx.EntityPlural+"Group", ctx.EntityName,
	)

	if !strings.Contains(content, ctx.EntityPlural+"Group") {
		content = insertBeforeReturn(content, groupLines)
	}

	return os.WriteFile(controllerPath, []byte(content), 0644)
}

func insertParam(content, line string) string {
	if out, ok := insertBeforeMarker(content, MarkerParams, line); ok {
		return out
	}
	// Insert before the closing ) of New(
	newIdx := strings.Index(content, "func New(")
	if newIdx < 0 {
		return content
	}
	depth := 0
	closeIdx := -1
	for i := newIdx; i < len(content); i++ {
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
	return content[:closeIdx] + line + "\n" + content[closeIdx:]
}

func insertRoute(content, line string) string {
	if out, ok := insertBeforeMarker(content, MarkerRoutes, line); ok {
		return out
	}
	retIdx := strings.LastIndex(content, "return router")
	if retIdx < 0 {
		return content
	}
	return content[:retIdx] + line + "\n\t" + content[retIdx:]
}

func insertBeforeReturn(content, lines string) string {
	if out, ok := insertBeforeMarker(content, MarkerRoutes, lines); ok {
		return out
	}
	retIdx := strings.LastIndex(content, "return router")
	if retIdx < 0 {
		return content
	}
	return content[:retIdx] + lines + content[retIdx:]
}

// PageAlias produces a unique Go import alias from a filesystem page path.
// "posts/_post_id" → "postsPostIdPage"
// "home" → "homePage"
func PageAlias(fsPagePath string) string {
	// Replace all non-alphanumeric chars (slashes, underscores) with separators
	re := regexp.MustCompile(`[/_]+`)
	clean := re.ReplaceAllString(fsPagePath, "_")
	clean = strings.Trim(clean, "_")
	parts := strings.Split(clean, "_")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			b.WriteString(strings.ToLower(p))
		} else {
			b.WriteString(toPascal(p))
		}
	}
	b.WriteString("Page")
	return b.String()
}

func routeRegistration(ctx TemplateContext, handlerVar string) string {
	// Determine the gin group path and the method to call
	ginPath := "/" + ctx.GinPath
	// Find the last segment that isn't a param — that's the group prefix
	segments := strings.Split(ctx.GinPath, "/")
	var groupSegments, paramSegments []string
	for _, s := range segments {
		if strings.HasPrefix(s, ":") {
			paramSegments = append(paramSegments, s)
		} else {
			groupSegments = append(groupSegments, s)
		}
	}
	_ = ginPath
	_ = paramSegments

	if len(ctx.ParamNames) == 0 {
		// Static route: r.GET("/path", handler.Handle)
		return fmt.Sprintf("\tr.GET(%q, %s.Handle)", "/"+ctx.GinPath, handlerVar)
	}
	// Dynamic route: use Register
	groupPath := "/" + strings.Join(groupSegments, "/")
	_ = groupPath
	return fmt.Sprintf("\t%s.Register(r.Group(%q))", handlerVar, "/"+strings.Join(groupSegments, "/"))
}
