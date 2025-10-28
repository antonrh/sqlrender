# Usage Guide

This page walks through the common ways people reach for SQLRender. Each section is short and to the point so you can copy, adjust, and ship.

## 1. Rendering inline strings

If you already have the SQL in your Go source, call `FromString`. The `bind` helper adds placeholders and collects parameters in order.

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

## 2. Loading templates from file

Prefer to keep SQL next to migrations or in a shared folder? Point the renderer at that directory and use `FromTemplate`.

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

`AddSearchPath` can be called multiple times. SQLRender walks the list until it finds the requested file.

## 3. Switching dialects

You can reuse the same template for multiple databases. Pass a different dialect when rendering and the placeholders adjust automatically.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectMySQL)

postgresSQL, postgresArgs, err := renderer.FromStringWithDialect(
	`WHERE created_at >= {{ bind .Start }}`,
	map[string]any{"Start": time.Now()},
	sqlrender.DialectPostgres,
)
```

Need schema-qualified names? The `identifier` helper quotes them safely:

```gotemplate
SELECT {{ identifier "public.users" }}.*
FROM {{ identifier "public.users" }}
```

## 4. Adding your own helper functions

Bring any helper logic you need straight into the template engine.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectSQLite).
	AddFunc("commaList", func(items []string) string {
		return strings.Join(items, ", ")
	})
```

Helpers registered via `AddFunc` or `AddFuncs` are available to every template rendered by that renderer.

## 5. Working with `database/sql`

Rendering is only half the story—here’s how you can send the result to a real driver. This example uses PostgreSQL, but any driver that matches the dialect works the same way.

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

stmt, err := db.Prepare(query)
if err != nil {
	return err
}
defer stmt.Close()

rows, err := stmt.Query(args...)
if err != nil {
	return err
}
defer rows.Close()
```

SQLRender focuses on creating valid SQL and argument slices. The database driver does the rest.

## 6. Error signals to watch for

- **“invalid identifier” panic** – `identifier` found characters outside the allowed set. Double-check the value passed into the template.
- **File not found** – `FromTemplate` lists the search paths it walked. Make sure the directory and filename are correct.
- **Template execution error** – bubbled up from `text/template` or a custom function. Fix the helper or the data passed in.

That’s it! Drop SQLRender into the parts of your code where raw string concatenation feels risky, and let it handle the boring pieces.
