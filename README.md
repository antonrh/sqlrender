# SQLRender

`SQLRender` is a small helper tool that lets you write SQL using Go templates.

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
