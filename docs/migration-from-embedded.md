# Migrating an application from an embedded `cmd/codegen`

This is a one-time extraction procedure for a project that still contains a local copy of the generator.

## 1. Add the standalone tool dependency

After the standalone repository has a release tag:

```bash
go get -tool github.com/dronm/codegen/cmd/codegen@v0.1.0
```

For simultaneous local development, add both modules to a `go.work` file instead of committing a local-path `replace`.

## 2. Add `codegen.yaml`

Move repository-level generator settings into a committed configuration file. Preserve environment variables only for secrets, CI switches, or temporary overrides.

Do not configure `templateDir` unless the application intentionally owns a template fork.

## 3. Remove the embedded generator implementation

Delete the application's old:

```text
cmd/codegen/
templates/
```

Do not delete the application's active `schema/` directory or generated `.gen.*` files.

## 4. Update scripts

Replace:

```bash
go run ./cmd/codegen
CODEGEN_CHECK=true go run ./cmd/codegen
```

with:

```bash
go tool codegen generate
go tool codegen check
```

## 5. Update documentation

Application documentation should describe only project-specific generator integration:

- where `codegen.yaml` lives;
- where active schemas live;
- project-specific backend/frontend registry hooks;
- which generator version the project pins.

The schema language and shared template behavior belong in the standalone generator repository.

## 6. Verify equivalence

Before changing schemas or templates, run generation with the extracted tool and inspect the diff. Existing generated source should remain unchanged except for intentionally changed generator behavior.

Then run:

```bash
go tool codegen check
go test ./...
```
