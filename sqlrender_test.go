package sqlrender

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"
)

func TestQueryArgsBindSequentialPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dialect    Dialect
		wantPhases []string
	}{
		{name: "postgres", dialect: DialectPostgres, wantPhases: []string{"$1", "$2", "$3"}},
		{name: "sqlserver", dialect: DialectSQLServer, wantPhases: []string{"@p1", "@p2", "@p3"}},
		{name: "oracle", dialect: DialectOracle, wantPhases: []string{":1", ":2", ":3"}},
		{name: "mysql", dialect: DialectMySQL, wantPhases: []string{"?", "?", "?"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			qa := NewQueryArgs(tt.dialect)

			got := []string{
				qa.Bind(1),
				qa.Bind(2),
				qa.Bind(3),
			}
			if !reflect.DeepEqual(got, tt.wantPhases) {
				t.Fatalf("placeholders mismatch: got %v, want %v", got, tt.wantPhases)
			}

			wantArgs := []any{1, 2, 3}
			if !reflect.DeepEqual(qa.args, wantArgs) {
				t.Fatalf("args mismatch: got %v, want %v", qa.args, wantArgs)
			}
		})
	}
}

func TestQueryArgsBindSliceAndArray(t *testing.T) {
	t.Parallel()

	qa := NewQueryArgs(DialectPostgres)

	gotSlice := qa.Bind([]int{4, 5})
	if gotSlice != "($1, $2)" {
		t.Fatalf("slice placeholder mismatch: got %q, want %q", gotSlice, "($1, $2)")
	}

	gotArray := qa.Bind([2]int{6, 7})
	if gotArray != "($3, $4)" {
		t.Fatalf("array placeholder mismatch: got %q, want %q", gotArray, "($3, $4)")
	}

	wantArgs := []any{4, 5, 6, 7}
	if !reflect.DeepEqual(qa.args, wantArgs) {
		t.Fatalf("args mismatch: got %v, want %v", qa.args, wantArgs)
	}
}

func TestQueryArgsBindEmptySlice(t *testing.T) {
	t.Parallel()

	qa := NewQueryArgs(DialectPostgres)
	got := qa.Bind([]string{})
	if got != "(NULL)" {
		t.Fatalf("empty slice placeholder mismatch: got %q, want %q", got, "(NULL)")
	}
	if len(qa.args) != 0 {
		t.Fatalf("expected no args to be bound, got %v", qa.args)
	}
}

func TestQueryArgsBindNil(t *testing.T) {
	t.Parallel()

	qa := NewQueryArgs(DialectPostgres)
	got := qa.Bind(nil)
	if got != "$1" {
		t.Fatalf("nil placeholder mismatch: got %q, want %q", got, "$1")
	}
	if qa.args[0] != nil {
		t.Fatalf("expected nil arg, got %v", qa.args[0])
	}
}

func TestQueryArgsIdentifierQuoting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dialect Dialect
		input   string
		want    string
	}{
		{"postgres schema", DialectPostgres, "public.users", `"public"."users"`},
		{"mysql simple", DialectMySQL, "users", "`users`"},
		{"sqlite dotted", DialectSQLite, "main.table", "`main`.`table`"},
		{"sqlserver bracket", DialectSQLServer, "dbo.People", `[dbo].[People]`},
		{"snowflake backtick", DialectSnowflake, "SNOW.TBL", "`SNOW`.`TBL`"},
		{"oracle quoted", DialectOracle, "HR.EMP", `"HR"."EMP"`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			qa := NewQueryArgs(tt.dialect)
			got := qa.Identifier(tt.input)
			if got != tt.want {
				t.Fatalf("identifier mismatch: got %q, want %q", got, tt.want)
			}
		})
	}

	qa := NewQueryArgs(DialectMySQL)
	if got := qa.Identifier(123); got != "" {
		t.Fatalf("expected empty identifier for non-string input, got %q", got)
	}
	if got := qa.Identifier(""); got != "" {
		t.Fatalf("expected empty identifier for empty string, got %q", got)
	}
}

func TestQueryArgsIdentifierInvalid(t *testing.T) {
	t.Parallel()

	qa := NewQueryArgs(DialectPostgres)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid identifier")
		}
	}()
	qa.Identifier("users;DROP")
}

func TestRendererSetDefaultDialect(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	if r.defaultDialect != DialectMySQL {
		t.Fatalf("initial dialect mismatch: got %q, want %q", r.defaultDialect, DialectMySQL)
	}

	out := r.SetDefaultDialect(DialectPostgres)
	if out != r {
		t.Fatal("SetDefaultDialect should return renderer instance")
	}
	if r.defaultDialect != DialectPostgres {
		t.Fatalf("updated dialect mismatch: got %q, want %q", r.defaultDialect, DialectPostgres)
	}
}

func TestRendererSearchPaths(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	r.AddSearchPath("one")
	r.AddSearchPath("two")

	if want := []string{"one", "two"}; !reflect.DeepEqual(r.searchPaths, want) {
		t.Fatalf("search paths mismatch: got %v, want %v", r.searchPaths, want)
	}

	out := r.SetSearchPaths([]string{"three"})
	if out != r {
		t.Fatal("SetSearchPaths should return renderer instance")
	}
	if want := []string{"three"}; !reflect.DeepEqual(r.searchPaths, want) {
		t.Fatalf("search paths mismatch after set: got %v, want %v", r.searchPaths, want)
	}
}

func TestRendererAddFuncs(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	r.AddFunc("one", func() string { return "one" })
	r.AddFuncs(template.FuncMap{
		"two": func() string { return "two" },
	})

	if _, ok := r.customFuncs["one"]; !ok {
		t.Fatal("expected custom func 'one'")
	}
	if _, ok := r.customFuncs["two"]; !ok {
		t.Fatal("expected custom func 'two'")
	}
}

func TestRendererAddFuncInitializesCustomMap(t *testing.T) {
	t.Parallel()

	var r Renderer
	r.AddFunc("x", func() string { return "x" })

	if r.customFuncs == nil {
		t.Fatal("expected customFuncs to be initialized")
	}
	if _, ok := r.customFuncs["x"]; !ok {
		t.Fatal("expected custom func 'x'")
	}
}

func TestRendererAddFuncsInitializesCustomMap(t *testing.T) {
	t.Parallel()

	var r Renderer
	r.AddFuncs(template.FuncMap{
		"x": func() string { return "x" },
	})

	if r.customFuncs == nil {
		t.Fatal("expected customFuncs to be initialized")
	}
	if _, ok := r.customFuncs["x"]; !ok {
		t.Fatal("expected custom func 'x'")
	}
}

func TestRendererFromStringWithDialect(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	sql, args, err := r.FromStringWithDialect(
		`SELECT {{ identifier "public.users" }} WHERE id IN {{ bind .IDs }}`,
		map[string]any{"IDs": []int{1, 2}},
		DialectPostgres,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `SELECT "public"."users" WHERE id IN ($1, $2)`
	if sql != wantSQL {
		t.Fatalf("sql mismatch: got %q, want %q", sql, wantSQL)
	}
	wantArgs := []any{1, 2}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch: got %v, want %v", args, wantArgs)
	}
}

func TestRendererFromStringWithDialectCustomFuncs(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	r.AddFunc("upper", strings.ToUpper)
	r.AddFuncs(template.FuncMap{
		"prefix": func(s string) string { return "p_" + s },
	})

	sql, args, err := r.FromStringWithDialect(
		`SELECT {{ upper (prefix .Name) }}`,
		map[string]any{"Name": "foo"},
		DialectMySQL,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT P_FOO" {
		t.Fatalf("sql mismatch: got %q, want %q", sql, "SELECT P_FOO")
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestRendererFromStringWithDialectParseError(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	if _, _, err := r.FromStringWithDialect(`{{`, nil, DialectMySQL); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestRendererFromStringWithDialectExecutionError(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	r.AddFunc("fail", func() (string, error) {
		return "", fmt.Errorf("boom")
	})

	if _, _, err := r.FromStringWithDialect(`{{ fail }}`, nil, DialectMySQL); err == nil {
		t.Fatal("expected execution error")
	}
}

func TestRendererFromStringUsesDefaultDialect(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	r.SetDefaultDialect(DialectSQLServer)

	sql, args, err := r.FromString(
		`WHERE id IN {{ bind .IDs }}`,
		map[string]any{"IDs": []int{1, 2}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `WHERE id IN (@p1, @p2)`
	if sql != wantSQL {
		t.Fatalf("sql mismatch: got %q, want %q", sql, wantSQL)
	}
	wantArgs := []any{1, 2}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch: got %v, want %v", args, wantArgs)
	}
}

func TestRendererFromTemplateWithDialectDirectPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(path, []byte(`SELECT {{ identifier "users" }}`), 0o600); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := NewRenderer(DialectMySQL)
	sql, args, err := r.FromTemplateWithDialect(path, nil, DialectPostgres)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT "users"` {
		t.Fatalf("sql mismatch: got %q, want %q", sql, `SELECT "users"`)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestRendererFromTemplateWithDialectReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "query.sql")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	r := NewRenderer(DialectMySQL)
	_, _, err := r.FromTemplateWithDialect(path, nil, DialectMySQL)
	if err == nil {
		t.Fatal("expected read error for directory path")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("error should mention read failure: %v", err)
	}
}

func TestRendererFromTemplateWithDialectSearchPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const filename = "query.sql"
	content := `WHERE id IN {{ bind .IDs }}`
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := NewRenderer(DialectMySQL)
	r.AddSearchPath(dir)

	sql, args, err := r.FromTemplateWithDialect(filename, map[string]any{"IDs": []int{10, 20}}, DialectOracle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `WHERE id IN (:1, :2)`
	if sql != wantSQL {
		t.Fatalf("sql mismatch: got %q, want %q", sql, wantSQL)
	}
	wantArgs := []any{10, 20}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch: got %v, want %v", args, wantArgs)
	}
}

func TestRendererFromTemplateNotFound(t *testing.T) {
	t.Parallel()

	r := NewRenderer(DialectMySQL)
	r.SetSearchPaths([]string{"missing_dir"})

	_, _, err := r.FromTemplateWithDialect("missing.sql", nil, DialectMySQL)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "missing.sql") {
		t.Fatalf("error should mention missing file: %v", err)
	}
	if !strings.Contains(err.Error(), "missing_dir") {
		t.Fatalf("error should mention search paths: %v", err)
	}
}

func TestRendererFromTemplateUsesDefaultDialect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const filename = "users.sql"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(`VALUES {{ bind .IDs }}`), 0o600); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	r := NewRenderer(DialectMySQL)
	r.SetDefaultDialect(DialectSQLServer)
	r.AddSearchPath(dir)

	sql, args, err := r.FromTemplate(filename, map[string]any{"IDs": []int{7, 8}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `VALUES (@p1, @p2)`
	if sql != wantSQL {
		t.Fatalf("sql mismatch: got %q, want %q", sql, wantSQL)
	}
	wantArgs := []any{7, 8}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch: got %v, want %v", args, wantArgs)
	}
}
