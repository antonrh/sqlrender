// Package sqlrender provides helpers for rendering SQL templates while safely
// binding arguments and quoting identifiers for a specific SQL dialect.
package sqlrender

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

// Dialect describes how placeholders and identifiers should be rendered for a
// specific database engine. The constants below cover the built-in dialects.
type Dialect string

const (
	DialectPostgres  Dialect = "postgres"
	DialectMySQL     Dialect = "mysql"
	DialectSQLite    Dialect = "sqlite"
	DialectSQLServer Dialect = "sqlserver"
	DialectSnowflake Dialect = "snowflake"
	DialectOracle    Dialect = "oracle"
)

// QueryArgs accumulates arguments to be bound into a SQL statement while
// keeping track of the dialect-specific placeholder format.
type QueryArgs struct {
	args    []any
	dialect Dialect
}

// NewQueryArgs returns a binder that formats placeholders for the supplied
// dialect.
func NewQueryArgs(dialect Dialect) *QueryArgs {
	return &QueryArgs{dialect: dialect}
}

// Bind stores the provided value and returns a placeholder string. Slice and
// array inputs expand into a comma-separated list wrapped in parentheses,
// while nil values map to a single placeholder.
func (qa *QueryArgs) Bind(arg any) string {
	v := reflect.ValueOf(arg)

	if !v.IsValid() {
		qa.args = append(qa.args, nil)
		return qa.placeholderFor(len(qa.args))
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		n := v.Len()
		if n == 0 {
			return "(NULL)"
		}

		placeholders := make([]string, n)
		for i := 0; i < n; i++ {
			val := v.Index(i).Interface()
			qa.args = append(qa.args, val)
			placeholders[i] = qa.placeholderFor(len(qa.args))
		}
		return fmt.Sprintf("(%s)", strings.Join(placeholders, ", "))
	default:
		qa.args = append(qa.args, arg)
		return qa.placeholderFor(len(qa.args))
	}
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._]+$`)

// Identifier quotes the supplied identifier (optionally schema-qualified) for
// the current dialect. Only alphanumeric characters, underscores, and periods
// are permitted; invalid identifiers trigger a panic to surface template issues
// early.
func (qa *QueryArgs) Identifier(name any) string {
	s, ok := name.(string)
	if !ok || s == "" {
		return ""
	}

	if !identifierPattern.MatchString(s) {
		panic(fmt.Sprintf("sqlrender: invalid identifier %q", s))
	}

	parts := strings.Split(s, ".")
	for i, part := range parts {
		parts[i] = qa.quoteIdentifier(part)
	}

	return strings.Join(parts, ".")
}

func (qa *QueryArgs) quoteIdentifier(id string) string {
	switch qa.dialect {
	case DialectPostgres, DialectOracle:
		return `"` + id + `"`
	case DialectSQLServer:
		return `[` + id + `]`
	default:
		return "`" + id + "`" // MySQL, SQLite, Snowflake
	}
}

func (qa *QueryArgs) placeholderFor(n int) string {
	switch qa.dialect {
	case DialectPostgres:
		return fmt.Sprintf("$%d", n)
	case DialectSQLServer:
		return fmt.Sprintf("@p%d", n)
	case DialectOracle:
		return fmt.Sprintf(":%d", n)
	default:
		return "?" // MySQL, SQLite, Snowflake
	}
}

// Renderer turns Go text templates into SQL statements while collecting the
// bound arguments.
type Renderer struct {
	searchPaths    []string
	defaultDialect Dialect
	customFuncs    template.FuncMap
}

// NewRenderer returns a Renderer that defaults to the provided dialect when no
// dialect override is specified during rendering.
func NewRenderer(defaultDialect Dialect) *Renderer {
	return &Renderer{
		defaultDialect: defaultDialect,
		customFuncs:    make(template.FuncMap),
	}
}

// SetDefaultDialect updates the renderer's default dialect and returns the
// renderer to allow fluent configuration.
func (r *Renderer) SetDefaultDialect(d Dialect) *Renderer {
	r.defaultDialect = d
	return r
}

// AddSearchPath appends a directory to the list of locations consulted when
// looking for template files.
func (r *Renderer) AddSearchPath(path string) *Renderer {
	r.searchPaths = append(r.searchPaths, path)
	return r
}

// SetSearchPaths replaces the search path list with the provided directories.
func (r *Renderer) SetSearchPaths(paths []string) *Renderer {
	r.searchPaths = paths
	return r
}

// AddFunc registers a single custom template function that will be available to
// all rendered templates.
func (r *Renderer) AddFunc(name string, fn any) *Renderer {
	if r.customFuncs == nil {
		r.customFuncs = make(template.FuncMap)
	}
	r.customFuncs[name] = fn
	return r
}

// AddFuncs registers several template functions at once, merging them with any
// existing custom functions.
func (r *Renderer) AddFuncs(funcs template.FuncMap) *Renderer {
	if r.customFuncs == nil {
		r.customFuncs = make(template.FuncMap)
	}
	for name, fn := range funcs {
		r.customFuncs[name] = fn
	}
	return r
}

// FromStringWithDialect renders the provided template string using the supplied
// dialect. It exposes the `bind` and `identifier` helper functions inside the
// template and returns both the rendered SQL and the collected arguments.
func (r *Renderer) FromStringWithDialect(
	s string,
	data map[string]any,
	dialect Dialect,
) (string, []any, error) {
	if data == nil {
		data = map[string]any{}
	}

	qa := NewQueryArgs(dialect)
	funcMap := template.FuncMap{
		"bind":       qa.Bind,
		"identifier": qa.Identifier,
	}

	for name, fn := range r.customFuncs {
		funcMap[name] = fn
	}

	tmpl, err := template.New("sql").Funcs(funcMap).Parse(s)
	if err != nil {
		return "", nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", nil, err
	}

	return buf.String(), qa.args, nil
}

// FromString renders a template string using the renderer's default dialect.
func (r *Renderer) FromString(s string, data map[string]any) (string, []any, error) {
	return r.FromStringWithDialect(s, data, r.defaultDialect)
}

// FromTemplateWithDialect loads the named template file, applying the search
// paths when necessary, and renders it using the supplied dialect.
func (r *Renderer) FromTemplateWithDialect(
	name string,
	data map[string]any,
	dialect Dialect,
) (string, []any, error) {
	path, err := r.findTemplateFile(name)
	if err != nil {
		return "", nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("sqlrender: failed to read %q: %w", path, err)
	}

	return r.FromStringWithDialect(string(content), data, dialect)
}

// FromTemplate renders the named template file using the renderer's default
// dialect.
func (r *Renderer) FromTemplate(name string, data map[string]any) (string, []any, error) {
	return r.FromTemplateWithDialect(name, data, r.defaultDialect)
}

func (r *Renderer) findTemplateFile(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return name, nil
	}

	for _, dir := range r.searchPaths {
		full := filepath.Join(dir, name)
		if _, err := os.Stat(full); err == nil {
			return full, nil
		}
	}

	return "", fmt.Errorf("sqlrender: template %q not found in search paths: %v", name, r.searchPaths)
}
