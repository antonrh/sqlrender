# Quick Start

Here’s a minimal example that renders a SQL query with placeholders and arguments ready for `database/sql`.

```go
renderer := sqlrender.NewRenderer(sqlrender.DialectPostgres)

sql, args, err := renderer.FromString(
	`SELECT * FROM users WHERE id IN {{ bind .UserIDs }}`,
	map[string]any{"UserIDs": []int{1, 2, 3}},
)
if err != nil {
	log.Fatal(err)
}

fmt.Println(sql)
fmt.Println(args)
```

**Output**

```text
SELECT * FROM users WHERE id IN ($1, $2, $3)
[1 2 3]
```

## How It Works

- `Templates`: write SQL using Go’s `text/template` syntax.
- `Helpers`: use built-in helpers like `bind` and `identifier`, or register your own.
- `Dialect Awareness`: SQLRender picks the right placeholder style for your database automatically.
- `Result`: get validated SQL plus an args slice ready for `database/sql`.

## When to Use SQLRender

- You want to avoid unsafe string concatenation in SQL queries.
- You prefer templated SQL files but still need correct placeholders and bindings.
- You maintain a codebase that targets multiple SQL dialects.
- You need parameterized, testable SQL without adding ORM complexity.

## Example: Use with database/sql

```go
db, err := sql.Open("postgres", os.Getenv("PG_DSN"))
if err != nil {
	log.Fatal(err)
}
defer db.Close()

renderer := sqlrender.NewRenderer(sqlrender.DialectPostgres)

query, args, err := renderer.FromString(
	`SELECT * FROM users WHERE email = {{ bind .Email }}`,
	map[string]any{"Email": "test@example.com"},
)
if err != nil {
	log.Fatal(err)
}

rows, err := db.Query(query, args...)
if err != nil {
	log.Fatal(err)
}
defer rows.Close()
```
