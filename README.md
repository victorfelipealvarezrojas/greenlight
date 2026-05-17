```markdown
# Greenlight

JSON REST API for retrieving and managing information about movies.

Built with Go following [Let's Go Further](https://lets-go-further.alexedwards.net/) by Alex Edwards.

## Stack

- Go 1.23
- PostgreSQL
- httprouter

## Project Structure

```
greenlight/
├── bin/          # Compiled binaries
├── cmd/
│   └── api/
│       └── main.go
├── internal/     # Internal packages (DB, validation, mail)
├── migrations/   # SQL migration files
├── remote/       # Production server configuration
├── go.mod
└── Makefile
```

## Tools

- `curl` — Manual HTTP testing
- `hey` — Load testing
- `git` — Version control

## Progress

| Chapter | Topic | Status |
|---------|-------|--------|
| 2.1 | Project setup and skeleton structure | ✅ |
```

