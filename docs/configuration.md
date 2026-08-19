# Configuration and CLI

## Commands

The executable accepts three commands:

```bash
codegen generate
codegen check
codegen validate
```

When installed as a Go tool dependency, use:

```bash
go tool codegen generate
go tool codegen check
go tool codegen validate
```

### `generate`

Loads and validates all active schemas, checks manual ownership collisions, then writes enabled backend/frontend output and migration drafts.

### `check`

Runs the same render pipeline without writing. Every managed generated target must already exist and match byte-for-byte. Existing manual Vue scaffolds only need to exist; their contents are developer-owned and are not compared with the original templates. Existing migration SQL is rendered and compared as well.

This is intended for CI.

### `validate`

Loads configuration and schemas, builds all object views, validates cross-object uniqueness, and performs manual backend/frontend collision checks. It does not render or compare generated files.

### CLI options

```text
-config path
```

Selects the configuration file. The default is `codegen.yaml` in the current directory.

Example:

```bash
go tool codegen validate -config ./config/codegen.yaml
```

When a non-default explicit config path does not exist, the command fails. For backward compatibility, absence of the default `codegen.yaml` falls back to the historical environment/default configuration.

## Configuration precedence

Configuration is assembled in this order, with later sources overriding earlier ones:

```text
built-in defaults
	↓
codegen.yaml
	↓
<project-root>/.env
	↓
<project-root>/.env.codegen
	↓
process environment
	↓
CLI command semantics (for example, `check` forces check mode)
```

`project-root` is the directory containing `codegen.yaml`. If the default file is absent, it is the current working directory.

All relative filesystem paths are resolved against `project-root`.

## `codegen.yaml`

Recommended baseline:

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
  createBin: migrate
  createExt: sql
  createSeq: true
  sequenceWidth: 6

allowManualCollisions: false
goJsonTagName: json
goFilterTagName: f
```

Unknown YAML properties are rejected.

### Top-level properties

| Property | Default | Meaning |
|---|---|---|
| `schemaDir` | `./schema` | Directory containing active object specifications. |
| `templateDir` | empty | Empty means use embedded templates. Set only to override the built-ins from disk. |
| `serverRoot` | `.` | Backend module root. |
| `frontendRoot` | `../front` | Vue application root when frontend generation is enabled. |
| `migrationsDir` | `./migrations` | Numbered migration directory. |
| `goModule` | read from `<serverRoot>/go.mod` | Explicit Go module path override. |
| `goJsonTagName` | `json` | Generated JSON struct-tag name. |
| `goFilterTagName` | `f` | Generated collection-filter struct-tag name. |
| `allowManualCollisions` | `false` | Disable manual ownership checks. Use only during deliberate migration. |

### Feature sections

| Property | Default | Meaning |
|---|---|---|
| `backend.enabled` | `true` | Generate Go models, services, HTTP routes. |
| `frontend.enabled` | `false` | Generate TypeScript/Valibot/API and enabled Vue scaffolds. |
| `apiTest.enabled` | `true` | Allow API integration-test generation when an object enables it. |
| `registries.enabled` | `true` | Generate backend and/or frontend registries. |
| `migrations.enabled` | `true` | Generate migration pairs for objects that enable migrations. |

### Migration section

| Property | Default | Meaning |
|---|---|---|
| `migrations.overwrite` | `false` | Rewrite an existing matching migration pair. Intended only for uncommitted drafts. |
| `migrations.createMode` | `internal` | `internal` allocates the next numeric sequence; `external` invokes a migration tool. |
| `migrations.createBin` | `migrate` | Executable used in external mode. |
| `migrations.createExt` | `sql` | Migration file extension. |
| `migrations.createSeq` | `true` | Pass `-seq` in external mode. |
| `migrations.sequenceWidth` | `6` | Width of internally allocated numeric prefixes. Must be 1..12. |

## Environment overrides

The previous `CODEGEN_*` contract remains supported:

| Variable | Corresponding configuration |
|---|---|
| `CODEGEN_SCHEMA_DIR` | `schemaDir` |
| `CODEGEN_TEMPLATE_DIR` | `templateDir` |
| `CODEGEN_SERVER_ROOT` | `serverRoot` |
| `CODEGEN_FRONTEND_ROOT` | `frontendRoot` |
| `CODEGEN_MIGRATIONS_DIR` | `migrationsDir` |
| `CODEGEN_GO_MODULE` | `goModule` |
| `CODEGEN_GO_JSON_TAG` | `goJsonTagName` |
| `CODEGEN_GO_FILTER_TAG` | `goFilterTagName` |
| `CODEGEN_BACKEND_ENABLED` | `backend.enabled` |
| `CODEGEN_FRONTEND_ENABLED` | `frontend.enabled` |
| `CODEGEN_APITEST_ENABLED` | `apiTest.enabled` |
| `CODEGEN_REGISTRIES_ENABLED` | `registries.enabled` |
| `CODEGEN_MIGRATIONS_ENABLED` | `migrations.enabled` |
| `CODEGEN_MIGRATIONS_OVERWRITE` | `migrations.overwrite` |
| `CODEGEN_ALLOW_MANUAL_COLLISIONS` | `allowManualCollisions` |
| `CODEGEN_MIGRATION_CREATE_MODE` | `migrations.createMode` |
| `CODEGEN_MIGRATION_CREATE_BIN` | `migrations.createBin` |
| `CODEGEN_MIGRATION_CREATE_EXT` | `migrations.createExt` |
| `CODEGEN_MIGRATION_CREATE_SEQ` | `migrations.createSeq` |
| `CODEGEN_MIGRATION_SEQUENCE_WIDTH` | `migrations.sequenceWidth` |
| `CODEGEN_CHECK` | legacy check-mode switch |

Boolean and integer environment values are validated. Invalid values cause the generator to fail instead of silently falling back.

`CODEGEN_CHECK=true` is retained for compatibility, but new scripts should prefer the explicit `check` command.

## Embedded templates

With no `templateDir`, templates are read from the generator executable itself. This has two useful properties:

1. the generator version and its templates are versioned together;
2. child projects cannot accidentally run a newer generator against stale copied templates.

A custom `templateDir` is resolved relative to `codegen.yaml` and must provide the normal `go/`, `sql/`, and `vue/` subtrees.


## Vue scaffold ownership migration

On the first `generate` after upgrading from the older generated-Vue ownership model, codegen renames legacy `*Form.gen.vue`, `*List.gen.vue`, and `*EditPage.gen.vue` files to their manually owned names when the destination does not already exist. This preserves any edits. If both names exist, generation fails and requires an explicit choice instead of deleting either file.
