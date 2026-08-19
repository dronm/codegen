# Codegen

`github.com/dronm/codegen` is a schema-driven CRUD code generator for Go/PostgreSQL backends and Vue 3 frontends.

It is intentionally opinionated. The built-in templates target the supported application stack and generate:

```text
YAML object specification
	↓
PostgreSQL migration draft
	↓
Go model/key/list models
	↓
Go CRUD service
	↓
HTTP routes and binders
	↓
permissions, application-route and menu seed data
	↓
optional API integration test
	↓
optional Vue 3 / TypeScript / Valibot / PrimeVue scaffold
```

Generated Vue collection pages support both page-based editing and native inline row editing through `@katren/vue-collection-lib`. Inline mode includes generated editable-column metadata, inline create drafts, and typed create payload mapping.

The generator is meant for ordinary database-backed CRUD and administrative entities. Domain workflows, transactions spanning several aggregates, parser logic, imports, image processing, and other application-specific behavior remain hand-written.

## Repository layout

```text
cmd/codegen/                 CLI entry point
internal/codegen/            generator implementation
internal/codegen/templates/  embedded default templates
docs/                        configuration and schema documentation
examples/                    reference object specifications
```

The templates are embedded into the executable with `go:embed`. A consuming project therefore needs only its schema files and `codegen.yaml`; it does not need a copied `templates/` directory.

## Add to a Go 1.24+ project

Pin the tool in the consuming module:

```bash
go get -tool github.com/dronm/codegen/cmd/codegen@v0.1.0
```

This adds a `tool` directive and the corresponding module requirement to the consumer's `go.mod`.

Run it from the consuming project:

```bash
go tool codegen generate
go tool codegen check
go tool codegen validate
```

`generate` is the default command, so `go tool codegen` is equivalent to `go tool codegen generate`.

## Local development before publishing a release

A Go workspace lets the generator and an application be developed together without changing either module's import paths.

Example workspace:

```text
dronm-workspace/
	apps/
		myapp/
	tools/
		codegen/
	go.work
```

Example `go.work`:

```go
// go.work
go 1.25.0

use (
	./apps/myapp
	./tools/codegen
)
```

The application still declares the tool normally in its `go.mod`. In workspace mode, Go uses the local `tools/codegen` module instead of downloading the tagged version.

After publishing/tagging the generator, projects can use the desired released version independently.

## Consumer configuration

Create `codegen.yaml` in the application root:

```yaml
schemaDir: ./schema
serverRoot: .
frontendRoot: ../front
migrationsDir: ./migrations

backend:
  enabled: true

frontend:
  enabled: false

apiTest:
  enabled: true

registries:
  enabled: true

migrations:
  enabled: true
  overwrite: false
  createMode: internal
  createExt: sql
  createSeq: true
  sequenceWidth: 6
```

All relative paths are resolved relative to the directory containing `codegen.yaml`, not relative to the process working directory.

The generator still supports the existing `CODEGEN_*` environment variables. They override `codegen.yaml`, which makes them suitable for CI or temporary local changes.

See [configuration](docs/configuration.md) for the complete precedence and option list.

## Object schemas

Put active `*.yaml` or `*.yml` object descriptions directly in the configured `schemaDir`. The generator does not recurse into subdirectories, so a `schema/examples/` directory can safely hold reference files.

Start with [examples/vehicle_brands.yaml](examples/vehicle_brands.yaml) for page editing or [examples/customers_inline.yaml](examples/customers_inline.yaml) for inline collection editing.

The complete schema contract is documented in [docs/schema-reference.md](docs/schema-reference.md).

## Generated-file ownership

Codegen deliberately uses two ownership modes. Files with `.gen` in their names are managed outputs and are the schema-driven source of truth; do not edit them manually. Vue list pages, edit pages, and forms are scaffolds: codegen creates them only when they do not exist, then leaves them under developer ownership.

For the frontend this means that types, schemas, APIs, form contracts, collection definitions, routes, and generated locale data remain synchronized with YAML, while `*List.vue`, `*Form.vue`, and `*EditPage.vue` can be reorganized and extended manually without regeneration overwriting the changes. A newly required field still changes the generated `*New` type, so a stale manual form is expected to fail TypeScript checking until it deliberately handles that field.

A typical CI check is:

```bash
go tool codegen check
go test ./...
```

`check` does not rewrite files. It fails when a managed generated target is missing or differs from the current schema/templates, or when an expected migration pair is missing/outdated. Existing manual Vue scaffolds are intentionally not compared with their original templates.

The generator deliberately refuses to overwrite conflicting hand-written model/service/API/frontend artifacts unless `allowManualCollisions` (or the corresponding environment override) is explicitly enabled.

When upgrading from the older `.gen.vue` ownership model, `generate` migrates a legacy `*Form.gen.vue`, `*List.gen.vue`, or `*EditPage.gen.vue` to the corresponding name without `.gen` when the manual target does not already exist, preserving any customizations. If both legacy and manual files exist, generation stops instead of deleting either file; keep the manual file and remove or archive the legacy one explicitly.

## Template overrides

The embedded templates are the default and should normally be used by all applications. If a project genuinely requires a temporary template fork, set:

```yaml
templateDir: ./codegen-templates
```

The directory must contain the same relative template layout as the built-ins:

```text
codegen-templates/
	go/
	sql/
	vue/
```

A template override is project-specific technical debt. Prefer evolving the shared generator when the change should apply to more than one application.

## Documentation

- [Configuration and CLI](docs/configuration.md)
- [Consumer integration](docs/integration.md)
- [Object schema reference](docs/schema-reference.md)
- [Extraction/migration guide](docs/migration-from-embedded.md)
