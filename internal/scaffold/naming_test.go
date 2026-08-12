package scaffold

import (
	"strings"
	"testing"
)

func TestToPascal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"get_user", "GetUser"},
		{"my-feature", "MyFeature"},
		{"seed_super_admin", "SeedSuperAdmin"},
		{"user", "User"},
		{"", ""},
		// Empty segments come from doubled or trailing separators and must not
		// produce an empty PascalCase chunk.
		{"get__user", "GetUser"},
		{"_user", "User"},
		{"user_", "User"},
		// Digits are not letters, so the segment they start is left alone.
		{"v1_handler", "V1Handler"},
	}
	for _, c := range cases {
		if got := toPascal(c.in); got != c.want {
			t.Errorf("toPascal(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToSnake(t *testing.T) {
	cases := []struct{ in, want string }{
		{"GetUser", "get_user"},
		{"User", "user"},
		{"", ""},
		// The i > 0 guard is what keeps the leading capital from producing a
		// leading underscore.
		{"HTTPServer", "h_t_t_p_server"},
	}
	for _, c := range cases {
		if got := toSnake(c.in); got != c.want {
			t.Errorf("toSnake(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToPackage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"MyFeature", "myfeature"},
		{"my-feature", "my_feature"},
		{"get_user", "get_user"},
		// Anything outside [a-z0-9_] is dropped, not replaced, so the result is
		// always a legal package name.
		{"user.profile", "userprofile"},
		{"user profile", "userprofile"},
	}
	for _, c := range cases {
		if got := toPackage(c.in); got != c.want {
			t.Errorf("toPackage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitGroup(t *testing.T) {
	cases := []struct{ in, wantGroup, wantName string }{
		{"get_profile", "", "get_profile"},
		{"users/get_profile", "users", "get_profile"},
		// Only the last segment is the name; everything before it stays the
		// group, however deep.
		{"admin/users/ban", "admin/users", "ban"},
		{"/users/get_profile/", "users", "get_profile"},
	}
	for _, c := range cases {
		group, name := SplitGroup(c.in)
		if group != c.wantGroup || name != c.wantName {
			t.Errorf("SplitGroup(%q) = (%q, %q), want (%q, %q)",
				c.in, group, name, c.wantGroup, c.wantName)
		}
	}
}

func TestPageAlias(t *testing.T) {
	cases := []struct{ in, want string }{
		{"home", "homePage"},
		{"posts/_post_id", "postsPostIdPage"},
		{"admin/users/_id", "adminUsersIdPage"},
	}
	for _, c := range cases {
		if got := PageAlias(c.in); got != c.want {
			t.Errorf("PageAlias(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// PageAlias exists to give every page package a unique wire import alias, so
// two different pages producing the same alias would be a silent collision in
// cmd/wire.go.
func TestPageAliasIsUniquePerPath(t *testing.T) {
	paths := []string{"home", "posts", "posts/_post_id", "admin/users", "admin/users/_id"}
	seen := map[string]string{}
	for _, p := range paths {
		alias := PageAlias(p)
		if prev, ok := seen[alias]; ok {
			t.Errorf("PageAlias collision: %q and %q both give %q", prev, p, alias)
		}
		seen[alias] = p
	}
}

// The user types one path and four derived forms have to agree, because each
// one lands in a different place: the directory, the template name and the gin
// route. Deriving them separately is what makes a table test worth having.
func TestPagePathForms(t *testing.T) {
	cases := []struct {
		in, gin, fs, view string
		params            []string
	}{
		{"home", "home", "home", "home", nil},
		{"posts/[post_id]", "posts/:post_id", "posts/_post_id", "posts/post_id", []string{"post_id"}},
		{
			"home/[user_id]/posts/[post_id]",
			"home/:user_id/posts/:post_id",
			"home/_user_id/posts/_post_id",
			"home/user_id/posts/post_id",
			[]string{"user_id", "post_id"},
		},
	}
	for _, c := range cases {
		if got := pagePathToGin(c.in); got != c.gin {
			t.Errorf("pagePathToGin(%q) = %q, want %q", c.in, got, c.gin)
		}
		if got := pagePathToFs(c.in); got != c.fs {
			t.Errorf("pagePathToFs(%q) = %q, want %q", c.in, got, c.fs)
		}
		if got := pagePathToView(c.in); got != c.view {
			t.Errorf("pagePathToView(%q) = %q, want %q", c.in, got, c.view)
		}
		got := extractParams(c.in)
		if len(got) != len(c.params) {
			t.Errorf("extractParams(%q) = %v, want %v", c.in, got, c.params)
			continue
		}
		for i := range got {
			if got[i] != c.params[i] {
				t.Errorf("extractParams(%q) = %v, want %v", c.in, got, c.params)
				break
			}
		}
	}
}

// Brackets are illegal in a Go import path, which is the entire reason
// FsPagePath exists alongside PagePath.
func TestFsPagePathHasNoBrackets(t *testing.T) {
	for _, p := range []string{"posts/[post_id]", "a/[b]/c/[d]"} {
		if got := pagePathToFs(p); strings.ContainsAny(got, "[]") {
			t.Errorf("pagePathToFs(%q) = %q, still contains a bracket", p, got)
		}
	}
}

// Import paths are slash-separated on every platform. These helpers use
// path.Join precisely so a Windows build does not emit backslashes.
func TestPkgPathsAreSlashSeparated(t *testing.T) {
	cases := []struct{ got, want string }{
		{UsecasePkgPathFor("", "get_user"), "internal/usecases/get_user"},
		{UsecasePkgPathFor("users", "user_get"), "internal/usecases/users/user_get"},
		{UsecasePkgPathFor("admin/users", "ban"), "internal/usecases/admin/users/ban"},
		{PagePkgPath("home"), "internal/web/pages/home"},
		{PagePkgPath("posts/_post_id"), "internal/web/pages/posts/_post_id"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
		if strings.Contains(c.got, `\`) {
			t.Errorf("%q contains a backslash; import paths are always slash-separated", c.got)
		}
	}
}

// Singularisation is deliberately naive — no irregulars — but it must not eat
// letters that are part of the stem. `notes` becoming `not` produced a
// domain.Not type and an ErrNotNotFound sentinel.
func TestCRUDSingularisation(t *testing.T) {
	project := NewProjectContext("app", "example.com/app", Features{})
	cases := []struct{ plural, entity, pascal string }{
		{"users", "user", "User"},
		{"posts", "post", "Post"},
		{"categories", "category", "Category"},
		{"boxes", "box", "Box"},
		{"matches", "match", "Match"},
		{"dishes", "dish", "Dish"},
		{"statuses", "status", "Status"},
		{"notes", "note", "Note"},
		{"images", "image", "Image"},
		{"types", "type", "Type"},
		{"order_items", "order_item", "OrderItem"},
	}
	for _, c := range cases {
		ctx := NewCRUDContext(project, c.plural)
		if ctx.EntityName != c.entity {
			t.Errorf("NewCRUDContext(%q).EntityName = %q, want %q", c.plural, ctx.EntityName, c.entity)
		}
		if ctx.PascalEntity != c.pascal {
			t.Errorf("NewCRUDContext(%q).PascalEntity = %q, want %q", c.plural, ctx.PascalEntity, c.pascal)
		}
		// CRUD packages are grouped under the plural, and the group is what
		// UsecasePkgPathFor nests them beneath.
		if ctx.GroupPath != c.plural {
			t.Errorf("NewCRUDContext(%q).GroupPath = %q, want %q", c.plural, ctx.GroupPath, c.plural)
		}
	}
}

func TestPascalFromContext(t *testing.T) {
	ctx := NewCRUDContext(NewProjectContext("app", "example.com/app", Features{}), "users")
	cases := []struct{ op, want string }{
		{"get", "GetUser"},
		{"list", "ListUser"},
		{"create", "CreateUser"},
	}
	for _, c := range cases {
		if got := PascalFromContext(ctx, c.op); got != c.want {
			t.Errorf("PascalFromContext(users, %q) = %q, want %q", c.op, got, c.want)
		}
	}
}

// Feature flags are copied by value from the project context, which is what
// lets `{{if .WithPostgres}}` work inside a template rendered for a use case
// generated long after `gostack new`.
func TestFeaturesPropagateToDerivedContexts(t *testing.T) {
	project := NewProjectContext("app", "example.com/app", Features{
		Frontend: true, Postgres: true, Redis: true, Docker: true,
	})
	derived := map[string]TemplateContext{
		"feature": NewFeatureContext(project, "users", "get_profile", false, true),
		"page":    NewPageContext(project, "posts/[post_id]"),
		"crud":    NewCRUDContext(project, "users"),
	}
	for name, ctx := range derived {
		if !ctx.WithFrontend || !ctx.WithPostgres || !ctx.WithRedis || !ctx.WithDocker {
			t.Errorf("%s context lost a feature flag: %+v", name, ctx)
		}
		if ctx.ModulePath != "example.com/app" || ctx.ProjectName != "app" {
			t.Errorf("%s context lost project identity: %+v", name, ctx)
		}
	}
}

// Every page package is `package page`, so the alias — not the package name —
// is what keeps them apart in cmd/wire.go.
func TestNewPageContext(t *testing.T) {
	ctx := NewPageContext(NewProjectContext("app", "example.com/app", Features{}), "posts/[post_id]")
	if ctx.PackageName != "page" {
		t.Errorf("PackageName = %q, want %q", ctx.PackageName, "page")
	}
	if ctx.FeatureName != "post_id" {
		t.Errorf("FeatureName = %q, want %q", ctx.FeatureName, "post_id")
	}
	if ctx.PascalName != "PostId" {
		t.Errorf("PascalName = %q, want %q", ctx.PascalName, "PostId")
	}
	if !ctx.HasPage || ctx.HasAPI {
		t.Errorf("HasPage = %v, HasAPI = %v, want true/false", ctx.HasPage, ctx.HasAPI)
	}
}
