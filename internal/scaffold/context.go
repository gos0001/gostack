package scaffold

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// WebDir is the project-relative root of the SSR layer: page packages live in
// WebDir/pages/<path>, the generated router in WebDir/routes.go. Everything
// under it renders HTML; everything in internal/usecases is backend logic.
const WebDir = "internal/web"

// Features are the optional components a project was created with.
// This struct is the single source of truth: it is persisted verbatim in
// gostack.json and mirrored into TemplateContext for template conditionals.
type Features struct {
	Frontend bool `json:"frontend"` // SSR pages + views + static + HTMX
	Postgres bool `json:"postgres"` // pkg/postgres + adapter + sqlc
	Redis    bool `json:"redis"`
	Docker   bool `json:"docker"`
}

// TemplateContext holds all template variables used during code generation.
type TemplateContext struct {
	// Project-level (set by `gostack new`)
	ProjectName string // "my-app"
	ModulePath  string // "github.com/user/my-app"

	// Feature-level (set by `gostack g *`)
	FeatureName  string // "get_user"       (raw, snake_case)
	PackageName  string // "get_user"       (safe Go package name)
	PascalName   string // "GetUser"        (for struct names)
	EntityName   string // "user"           (for CRUD: singular entity)
	EntityPlural string // "users"          (for CRUD: plural / table name)
	PascalEntity string // "User"           (for CRUD: domain type name)

	// GroupPath nests a usecase under a domain folder. "" means flat.
	// CRUD sets it to EntityPlural; `g uc users/get_profile` sets "users".
	GroupPath string

	// Page routing (set by `gostack g p`)
	PagePath   string   // "posts/[post_id]"  (user-facing, Next.js style)
	FsPagePath string   // "posts/_post_id"   (filesystem-safe, valid Go import path)
	ViewPath   string   // "posts/post_id"    (template name for views.Render)
	GinPath    string   // "posts/:post_id"   (gin-compatible route)
	ParamNames []string // ["post_id"]

	// Flags
	HasPage bool // generate page.go + template
	HasAPI  bool // generate http_v1.go

	// Project features, mirrored from Features so templates can branch on
	// {{if .WithPostgres}}. Every constructor copies the project context by
	// value, so these propagate to feature/page/CRUD contexts automatically.
	WithFrontend bool
	WithPostgres bool
	WithRedis    bool
	WithDocker   bool
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9_]`)

// toPascal converts snake_case or kebab-case to PascalCase.
// "get_user" → "GetUser", "my-feature" → "MyFeature"
func toPascal(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

// toPackage converts a name to a valid Go package name (lowercase, no special chars).
// "MyFeature" → "myfeature", "my-feature" → "myfeature"
func toPackage(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "_")
	return nonAlphaNum.ReplaceAllString(s, "")
}

// toSnake converts PascalCase to snake_case.
// "GetUser" → "get_user"
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteRune('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// pagePathToGin converts Next.js-style path to gin route.
// "home/[user_id]" → "home/:user_id"
func pagePathToGin(path string) string {
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	return re.ReplaceAllString(path, ":$1")
}

// extractParams extracts param names from a page path.
// "home/[user_id]/posts/[post_id]" → ["user_id", "post_id"]
func extractParams(path string) []string {
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	matches := re.FindAllStringSubmatch(path, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// NewProjectContext builds a TemplateContext for `gostack new`.
func NewProjectContext(projectName, modulePath string, f Features) TemplateContext {
	return TemplateContext{
		ProjectName:  projectName,
		ModulePath:   modulePath,
		HasPage:      f.Frontend,
		HasAPI:       true,
		WithFrontend: f.Frontend,
		WithPostgres: f.Postgres,
		WithRedis:    f.Redis,
		WithDocker:   f.Docker,
	}
}

// NewFeatureContext builds a TemplateContext for `gostack g uc/api`.
// group is "" for a flat usecase, or a domain folder like "users".
func NewFeatureContext(project TemplateContext, group, featureName string, hasPage, hasAPI bool) TemplateContext {
	ctx := project
	ctx.GroupPath = group
	ctx.FeatureName = featureName
	ctx.PackageName = toPackage(featureName)
	ctx.PascalName = toPascal(featureName)
	ctx.HasPage = hasPage
	ctx.HasAPI = hasAPI
	return ctx
}

// pagePathToFs converts Next.js-style path to filesystem-safe path.
// "posts/[post_id]" → "posts/_post_id"
// Brackets are replaced with underscore prefix; the result is a valid Go import path.
func pagePathToFs(path string) string {
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	return re.ReplaceAllString(path, "_$1")
}

// pagePathToView converts page path to the template view name.
// "posts/[post_id]" → "posts/post_id"
func pagePathToView(path string) string {
	re := regexp.MustCompile(`\[([^\]]+)\]`)
	return re.ReplaceAllString(path, "$1")
}

// NewPageContext builds a TemplateContext for `gostack g p`.
func NewPageContext(project TemplateContext, pagePath string) TemplateContext {
	ctx := project
	ctx.PagePath = pagePath
	ctx.FsPagePath = pagePathToFs(pagePath)
	ctx.ViewPath = pagePathToView(pagePath)
	ctx.GinPath = pagePathToGin(pagePath)
	ctx.ParamNames = extractParams(pagePath)
	// Derive a feature name from the last segment (strip param brackets).
	last := pagePath
	if idx := strings.LastIndex(pagePath, "/"); idx >= 0 {
		last = pagePath[idx+1:]
	}
	re := regexp.MustCompile(`[\[\]]`)
	last = re.ReplaceAllString(last, "")
	ctx.FeatureName = last
	ctx.PackageName = "page"
	ctx.PascalName = toPascal(last)
	ctx.HasPage = true
	ctx.HasAPI = false
	return ctx
}

// PascalFromContext builds a PascalCase name for a CRUD operation.
// ctx.PascalEntity = "User", op = "get" → "GetUser"
func PascalFromContext(ctx TemplateContext, op string) string {
	return toPascal(op) + ctx.PascalEntity
}

// SplitGroup splits an optional group prefix off a feature argument.
//
//	"users/get_profile" → ("users", "get_profile")
//	"get_profile"       → ("",      "get_profile")
//
// Multi-level groups ("admin/users/ban") are preserved in the group half.
func SplitGroup(arg string) (group, name string) {
	arg = strings.Trim(arg, "/")
	idx := strings.LastIndex(arg, "/")
	if idx < 0 {
		return "", arg
	}
	return arg[:idx], arg[idx+1:]
}

// UsecasePkgPath returns the module-relative Go import path of a usecase.
func UsecasePkgPath(ctx TemplateContext) string {
	return UsecasePkgPathFor(ctx.GroupPath, ctx.PackageName)
}

// UsecasePkgPathFor builds a usecase import path. It uses path.Join, never
// filepath.Join — Go import paths are always slash-separated, even on Windows.
//
//	group ""      → "internal/usecases/get_user"
//	group "users" → "internal/usecases/users/user_get"
func UsecasePkgPathFor(group, pkg string) string {
	if group == "" {
		return path.Join("internal", "usecases", pkg)
	}
	return path.Join("internal", "usecases", group, pkg)
}

// PagePkgPath returns the module-relative Go IMPORT path of a page package.
// Always slash-separated — path.Join, never filepath.Join.
func PagePkgPath(fsPagePath string) string {
	return path.Join(WebDir, "pages", fsPagePath)
}

// PageDir returns the OS FILESYSTEM path of a page package inside a project.
// Deliberately adjacent to PagePkgPath: the two differ only on Windows, which
// is exactly why mixing them up goes unnoticed until someone builds there.
func PageDir(fsPagePath string) string {
	return filepath.FromSlash(PagePkgPath(fsPagePath))
}

// NewCRUDContext builds a TemplateContext for `gostack g crud`.
func NewCRUDContext(project TemplateContext, entityPlural string) TemplateContext {
	ctx := project
	ctx.EntityPlural = entityPlural
	// Strip trailing 's' for simple singularisation ("users" → "user")
	entity := entityPlural
	if strings.HasSuffix(entity, "ies") {
		entity = entity[:len(entity)-3] + "y"
	} else if strings.HasSuffix(entity, "es") {
		entity = entity[:len(entity)-2]
	} else if strings.HasSuffix(entity, "s") {
		entity = entity[:len(entity)-1]
	}
	ctx.EntityName = entity
	ctx.PascalEntity = toPascal(entity)
	// CRUD packages are grouped under a folder named after the entity.
	ctx.GroupPath = entityPlural
	ctx.HasAPI = true
	return ctx
}
