# Backend enums

A top-level `enums` definition is now sufficient to generate a named Go type,
typed constants, value helpers, and modelbind registration. This fixes models
that reference `MaterialStatusType` without any Go declaration for that type.
The same definition continues to drive frontend types, validation, and columns.

## Existing MaterialStatus YAML

No additional backend setting is required:

```yaml
enums:
  - name: material_status_type
    goType: MaterialStatusType
    sqlType: public.material_status_types
    frontendName: MaterialStatusType
    values:
      - value: at_work
        label: В работе
      - value: on_maintenance
        label: На обслуживании

fields:
  - name: status
    type: enum
    enum: material_status_type
    required: true
    frontendDefault: at_work
```

Use the same `type` and `enum` in matching list/detail projection fields. Backend
types and `enum:"material_status_type"` tags remain consistent across them;
nullable fields use `*MaterialStatusType` and must not also set `required: true`.

From the consuming project's root:

```bash
go tool codegen validate
go tool codegen generate
go build ./...
```

With a local Go module replacement, run those commands after updating the local
Codegen source. The generated Go files must be committed with the application.

## Generated artifacts

With `backend.enabled: true`, this definition creates:

```text
internal/models/materialStatusType.enum.gen.go
internal/models/enums_registry.gen.go
.codegen/backend-enums.json
```

The manifest is relative to `serverRoot`, not `frontendRoot`. Commit it along with
the generated files. Definitions shared by multiple loaded objects are emitted
only once. Declarations that are defined but currently unused are still emitted.

The per-enum API is:

```go
type MaterialStatusType string

const (
	MaterialStatusTypeAtWork        MaterialStatusType = "at_work"
	MaterialStatusTypeOnMaintenance MaterialStatusType = "on_maintenance"
)

// A new slice on every call; callers cannot change subsequent results.
func MaterialStatusTypeValues() []string

func (v MaterialStatusType) IsValid() bool
func (v MaterialStatusType) String() string
```

The method signatures above summarize the API, rather than forming a standalone
Go implementation. JSON retains ordinary named-string behavior; nullable
pointers encode/decode `null`. Decoding JSON alone does not validate membership:
use modelbind validation on requests or `IsValid()` for application-created
values. No custom JSON, SQL scanner, or driver conversion is imposed on existing
projects.

Constant names default to `goType` followed by the PascalCase enum value.
Punctuation separates words; serialized values themselves are unchanged. A name
collision, such as `at_work` versus `at-work`, requires an explicit constant name:

```yaml
values:
  - value: at_work
    label: В работе
  - value: at-work
    label: Старое значение
    goConst: MaterialStatusTypeAtWorkLegacy
```

`goConst` is the complete Go identifier, not a suffix. Generated names may not
collide with types, helpers, other managed enums, or application declarations.

## Modelbind startup registration

`enums_registry.gen.go` contains `RegisterGeneratedModelbindEnums()` and an
`init()` function that calls it. Normal applications need no new startup hook.
The function initializes `metadata.Enums` when nil and merges only the generated
registry entries, preserving unrelated manually registered enums. Repeating it
with identical values is supported. A conflicting entry causes a startup panic
with the registry name and expected/actual values, rather than silently replacing
another owner's values. All conflicts are checked before any entries are added.

Keep existing application-specific registration functions. Do not replace the
entire `metadata.Enums` map after package initialization. If existing startup code
does replace the map, call this helper immediately afterward:

```go
models.RegisterGeneratedModelbindEnums()
```

This must happen before parsing/caching model metadata or starting concurrent
request handling. The helper is a startup operation, not a concurrent registry
mutation API. Later application code can still overwrite map entries; Codegen
cannot prevent arbitrary writes to another package's public global map.

`backend.register` controls enum registration independently of the existing
project-level `registries.enabled` setting for routes and services. Setting
`backend.enabled: false` suppresses both enum declarations and registration;
it does not delete previously generated backend files.

## Ownership and compatibility

The explicit form of the default is:

```yaml
enums:
  - name: material_status_type
    goType: MaterialStatusType
    sqlType: public.material_status_types
    frontendName: MaterialStatusType
    backend:
      generation: managed
      register: true
    values:
      - value: at_work
        label: В работе
      - value: on_maintenance
        label: На обслуживании
```

For an existing developer-owned Go enum, preserve it with:

```yaml
backend:
  generation: manual
  register: false
```

`generation` accepts `managed` or `manual`. When omitted it is `managed`.
`register` defaults to true for managed declarations and false for manual ones.
A manual type can explicitly use `register: true`; registration then uses the
YAML literals and does not require any particular helper in the manual Go file.
A managed type can use `register: false` when registration remains manual.

Codegen does not rewrite an existing `RegisterModelbindEnums()` function or any
manual enum file. Duplicate type/constant/helper/method declarations are reported
before enum output is written. `allowManualCollisions` deliberately does not
bypass enum duplicate-declaration safeguards.

If you added `internal/models/materialStatusType.go` as a workaround, either
remove that duplicate deliberately and let Codegen own the type, or set
`backend.generation: manual`. Do not keep two declarations of the same Go type.

Legacy backend-only fields supplying `type: enum`, `goType`, and a registry name
without a top-level definition retain their previous manual ownership. Merely
using an enum tag does not generate a new type or registry entry.

## Optional initial PostgreSQL enum migrations

PostgreSQL type ownership is a separate, explicit decision. By default, no enum
DDL is generated: `sqlType` continues to specify the column type and the
application must already provide that SQL type where it is used.

For a new, Codegen-owned type and a new migration history, add this inside its
enum definition:

```yaml
migration:
  enabled: true
  # Optional; this is also the default name for the sample sqlType.
  name: create_enum_public_material_status_types
```

This requires a schema-qualified `sqlType`. The schema must already exist.
With project-level `migrations.enabled: true`, Codegen creates a dedicated shared
migration before generating dependent table migrations:

```sql
BEGIN;

CREATE TYPE public.material_status_types AS ENUM (
	'at_work',
	'on_maintenance'
);

COMMIT;
```

The down migration uses `DROP TYPE public.material_status_types RESTRICT`, never
`CASCADE`. Reverse dependent table/column migrations before dropping a shared type.
No `IF NOT EXISTS` or exception suppression hides an existing incompatible type.
Use `migration.enabled: false` for a type owned by existing application migrations.

Existing enum migration pairs are immutable, even with the global migrations
overwrite option. `check` checks their presence and dependency order, not whether
historical enum values match today's schema. Missing half-pairs are errors.
Deleting a definition does not delete its migration files or drop the SQL type.

Codegen refuses to append an initial enum migration after an existing dependent
table migration: that would break clean replay on an empty database. For an
existing database, leave enum migration generation disabled and write a reviewed
forward migration, or explicitly arrange dependency order only in an unapplied
history. Codegen never renumbers or retroactively modifies applied migrations.

Changing an existing `status text` column into an enum still needs a reviewed
`ALTER TABLE` migration. Check current values, defaults, dependent views, and
constraints first; this generator does not connect to the database or infer
those transformations. A `frontendDefault` is not a PostgreSQL default.

## Evolving enums

Updating YAML regenerates managed Go/TypeScript artifacts and validation values.
It does not edit historical `CREATE TYPE` migrations. Add a new migration for
SQL changes, for example:

```sql
ALTER TYPE public.material_status_types ADD VALUE 'retired';
```

A new enum value added within a transaction cannot be used until that transaction
commits. Plan subsequent data/default changes in a later transaction. Removing
or reordering existing enum values requires deliberate data/schema migration;
there is no automatically generated destructive rollback for an added value.

PostgreSQL references: [CREATE TYPE](https://www.postgresql.org/docs/current/sql-createtype.html)
and [ALTER TYPE](https://www.postgresql.org/docs/current/sql-altertype.html).
Modelbind's [startup and enum-tag contract](https://github.com/dronm/modelbind)
explains the `metadata.Enums` integration.

## Managed-file checks and tests

`check` reports missing, changed, or stale backend enum files without writing.
`generate` removes only obsolete files recorded in `.codegen/backend-enums.json`.
Stale files with changed content or missing ownership markers are not deleted.
Untracked manual files are never swept. Symlinked output files/directories and
unsafe manifest paths are refused. Custom enum templates must retain both
ownership header lines from the built-in templates.

The existing templates for models, reference columns, labels, datetime, forms,
and frontend enums are unchanged by this feature.

Run `go test ./...` and `go vet ./...` in Codegen. Backend tests cover enum
rendering, helpers/JSON, registry merge behavior, manual ownership, names,
collisions, shared definitions, cleanup, and migration ordering. The generated
registry unit test substitutes an in-memory map for `metadata.Enums`; it is not
an integration test of modelbind's request validator. The real YAML/CLI test is
`TestBackendEnumYAMLGenerationIntegration`. Finally, regenerate and build the
consuming application with its real modelbind version and test its API validation.
