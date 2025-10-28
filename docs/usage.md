# Usage Guide

This guide covers the most common ways to use SQLRender. Each example is short and practical so you can copy, adapt, and reuse it immediately.

## 1. Render Inline Strings

If your SQL is defined directly in Go code, use `FromString`. The `bind` helper replaces placeholders and collects parameters in order.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectPostgres)

sqlText, args, err := renderer.FromString(
	`SELECT * FROM accounts WHERE id = {{ bind .ID }}`,
	map[string]any{"ID": 42},
)
if err != nil {
	log.Fatal(err)
}
// sqlText => "SELECT * FROM accounts WHERE id = $1"
// args    => []any{42}
```

## 2. Load Templates from Files

To keep SQL in separate files (next to migrations or shared queries), use `FromTemplate` and specify one or more search paths.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectMySQL).
	AddSearchPath("sql/templates")

sqlText, args, err := renderer.FromTemplate("reports/monthly.sql", map[string]any{
	"Months": []int{1, 2, 3},
})
if err != nil {
	log.Fatal(err)
}
```

You can call `AddSearchPath` multiple times. SQLRender searches each directory until it finds the requested file.

## 3. Switch Dialects

The same template can be reused across multiple databases. Specify a different dialect when rendering, and SQLRender adjusts placeholders automatically.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectMySQL)

postgresSQL, postgresArgs, err := renderer.FromStringWithDialect(
	`WHERE created_at >= {{ bind .Start }}`,
	map[string]any{"Start": time.Now()},
	sqlrender.DialectPostgres,
)
```

To safely quote schema or table names, use the `identifier` helper:

```sql
SELECT {{ identifier "public.users" }}.*
FROM {{ identifier "public.users" }}
```

## 4. Add Helper Functions

Add custom logic to templates with `AddFunc` or `AddFuncs`.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectSQLite).
	AddFunc("commaList", func(items []string) string {
		return strings.Join(items, ", ")
	})
```

All registered helpers are available to every template rendered by that renderer instance.

## 5. Work with database/sql

Once SQLRender generates the query text and arguments, execute them directly with any `database/sql` driver.

```go
import (
	"database/sql"
	"os"

	_ "github.com/lib/pq"
)

renderer := sqlrender.NewRenderer(sqlrender.DialectPostgres)

query, args, err := renderer.FromString(
	`SELECT * FROM users WHERE id IN {{ bind .IDs }} AND active = {{ bind .Active }}`,
	map[string]any{"IDs": []int{1, 2, 3}, "Active": true},
)
if err != nil {
	return err
}

db, err := sql.Open("postgres", os.Getenv("PG_DSN"))
if err != nil {
	return err
}
defer db.Close()

rows, err := db.Query(query, args...)
if err != nil {
	return err
}
defer rows.Close()
```

SQLRender focuses on producing valid SQL and argument slices — the driver handles the rest.

## 6. Handle Errors

Common error signals include:

- `invalid identifier panic`: the `identifier` helper detected invalid characters. Check the input string.
- `file not found`: `FromTemplate` lists all paths it searched. Verify the directory and filename.
- `template execution error`: an error occurred in `text/template` or a custom helper. Check the template logic or data.
