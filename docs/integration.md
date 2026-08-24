# Consumer integration

This document describes what a child application owns and how it invokes the standalone generator.

## Ownership split

The generator repository owns:

```text
CLI and generator implementation
schema parser and validation
naming and collision rules
built-in Go/SQL/Vue templates
versioned common register runtime templates
schema documentation and examples
```

Each application owns:

```text
codegen.yaml
schema/*.yaml
generated .gen.* source files
generated migration drafts
hand-written domain extensions
schema/registers/*.yaml
```

The application must not copy `cmd/codegen` or the default templates into its repository.

## Go tool dependency

Go 1.24+ supports executable module dependencies directly in `go.mod`.

Add the generator with:

```bash
go get -tool github.com/dronm/codegen/cmd/codegen@v0.1.0
```

Conceptually, the resulting `go.mod` contains:

```go
require github.com/dronm/codegen v0.1.0

tool github.com/dronm/codegen/cmd/codegen
```

Use the tool by its final path component:

```bash
go tool codegen generate
```

The version is selected through the normal module graph, so different projects may remain on different generator releases.

## Recommended Make targets

```make
.PHONY: codegen codegen-check codegen-validate

codegen:
	@go tool codegen generate

codegen-check:
	@go tool codegen check

codegen-validate:
	@go tool codegen validate
```

## Local development with a Go workspace

When changing the generator and a consumer at the same time, put both modules in a `go.work` file:

```go
go 1.25.0

use (
	./apps/myapp
	./tools/codegen
)
```

The application's `go.mod` remains versioned normally; workspace mode substitutes the local module for development.

This is preferable to committing a local-path `replace` directive in every application.

## Required backend hooks

The generated registry functions must be called by the application once during startup.

The current built-in backend profile generates:

```text
internal/httpapi/routes_gen.go
internal/services/services_gen.go
```

The application should have a hand-written route setup that calls:

```go
registerGeneratedRoutes(&api)
```

and service registration that calls:

```go
services.RegisterGeneratedServices()
```

These hook calls are application integration points; they are not generated because startup layout is application-owned.

## Register integration

When register schemas are present, backend generation writes typed helpers and a common registry under:

```text
internal/registers/*.gen.go
```

These files are ordinary managed generated output. Document services should call the generated recorder lock and concrete add/remove helpers using the same `ds.Querier` transaction that saves the document and its lines. Recorder-specific document-to-action mapping stays in hand-written service extensions.

The first generated register also causes the pinned common runtime migration to be created before its concrete register migration. Commit both migration pairs. An application does not import SQL or templates from Codegen at runtime.

See [Accumulation registers](registers.md) for posting, interval, rebuild, and migration ownership rules.

## Frontend integration

When frontend generation is enabled, the built-in profile writes:

```text
src/router/routeManifest.gen.ts
src/locales/ru.gen.json
```

The application should merge/import those generated registries from its hand-written router and i18n bootstrap. The exact bootstrap remains application-owned.

## CI

Commit generated files and run:

```bash
go tool codegen check
go test ./...
```

If frontend output is enabled, also run the consumer's normal frontend typecheck/build.

The check command detects drift between:

```text
schema + generator version + templates
```

and the committed managed generated files. Vue `*List.vue`, `*Form.vue`, and `*EditPage.vue` scaffolds are only required to exist; their contents are intentionally developer-owned after first generation.


## Vue scaffold ownership migration

On the first `generate` after upgrading from the older generated-Vue ownership model, codegen renames legacy `*Form.gen.vue`, `*List.gen.vue`, and `*EditPage.gen.vue` files to their manually owned names when the destination does not already exist. This preserves any edits. If both names exist, generation fails and requires an explicit choice instead of deleting either file.
