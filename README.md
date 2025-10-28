# SQLRender

SQLRender is a tiny helper for people who like SQL templates but dislike hand-crafted placeholder lists. Point it at a dialect, feed it a Go `text/template`, and it returns ready-to-run SQL plus the arguments slice you can give to `database/sql`.

## What SQLRender handles

- Picks the right placeholder style for PostgreSQL, MySQL, SQLite, SQL Server, Snowflake, and Oracle.
- Expands slices and arrays so `IN` clauses are painless.
- Quotes identifiers safely and fails fast if a name looks suspicious.
- Lets you share templates via search paths and custom helper functions.

## Install

```bash
go get github.com/antonrh/sqlrender
```

## Quick start

```go
package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/antonrh/sqlrender"
)

func main() {
	renderer := sqlrender.NewRenderer(sqlrender.DialectPostgres)

	query, args, err := renderer.FromString(
		`SELECT * FROM users WHERE id IN {{ bind .IDs }} AND active = {{ bind .Active }}`,
		map[string]any{"IDs": []int{1, 2, 3}, "Active": true},
	)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("postgres", os.Getenv("PG_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}
```

### Rendering templates from file

Keep SQL files in version control? Add a search path and call `FromTemplate`.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectPostgres).
	AddSearchPath("sql")

query, args, err := renderer.FromTemplate("reports/active_users.sql", map[string]any{
	"Regions": []string{"us", "eu"},
})
if err != nil {
	log.Fatal(err)
}
```
