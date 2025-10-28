# SQLRender

SQLRender keeps SQL templates friendly without giving up on safety. Feed it a Go `text/template`, point it at a SQL dialect, and it takes care of building placeholders, quoting identifiers, and collecting arguments for you.

## Why SQLRender?

- Works across PostgreSQL, MySQL, SQLite, SQL Server, Snowflake, and Oracle.
- Turns slices and arrays into proper `IN (...)` lists with the right placeholders.
- Quotes identifiers only if they contain safe characters, so mistakes surface early.
- Lets you drop in your own helper functions or load templates from disk.

## Installation

```bash
go get github.com/antonrh/sqlrender
```

Use your own module path if you publish a fork.

## Quick peek

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

Output:

```
SELECT * FROM users WHERE id IN ($1, $2, $3)
[1 2 3]
```

Head to the [usage guide](usage.md) when you’re ready for more examples.
