# Accumulation registers

Codegen can generate monthly accumulation registers for balances such as material stock, fuel, or working time. A register is separate from an ordinary CRUD object: its action ledger is immutable, its aggregates are maintained automatically, and it has no generated HTTP CRUD or Vue editor.

## Architecture

Register generation has three ownership layers:

| Layer | Owner | Examples |
|---|---|---|
| Common register runtime | Codegen, copied once into the project | Business timezone and month-boundary functions |
| Concrete register | Generated from register YAML | `ra_materials`, `rg_materials_period`, balance and summary functions |
| Recorder posting | Application | Convert a receipt, consumption, or transfer into signed actions |

The database trigger owns only the invariant from an action to its aggregates. The application explicitly owns the document-to-action mapping and calls it inside the document transaction.

## Configuration

Register specifications live in a directory separate from object schemas:

```yaml
schemaDir: ./schema
registers:
  enabled: true
  schemaDir: ./schema/registers
  businessTimezone: Asia/Yekaterinburg
  runtimeVersion: 1
```

The default register directory is `./schema/registers`. A missing directory is treated as an empty register set. When there are no active register schemas, no runtime migration or register files are generated.

`runtimeVersion` is pinned in the project configuration. Runtime version 1 is the only version supported by this release.

## Register schema

Put register `*.yaml` or `*.yml` files directly in `registers.schemaDir`. Codegen does not recurse into its subdirectories.

```yaml
name: Materials
comment: Material stock by construction site.
kind: accumulation
period: month

dimensions:
  - name: construction_site_id
    type: int
    references:
      schema: public
      table: construction_sites
      column: id
      onDelete: RESTRICT

  - name: material_id
    type: int
    references:
      schema: public
      table: materials
      column: id
      onDelete: RESTRICT

resources:
  - name: quant
    type: numeric
    sqlType: numeric(19, 4)

migration:
  enabled: true
  name: create_materials_register
```

The complete example is in [`examples/materials_register.yaml`](../examples/materials_register.yaml).

### Top-level properties

| Property | Default | Meaning |
|---|---|---|
| `name` | required | Go/register name. `Materials` produces the default table base `materials`. |
| `comment` | generated description | Register description. |
| `schema` | `public` | PostgreSQL schema for the concrete register. |
| `tableName` | snake case of `name` | Base used for `ra_*` and `rg_*` names. |
| `kind` | `accumulation` | Version 1 supports only accumulation registers. |
| `period` | `month` | Version 1 supports only monthly aggregation. |
| `dimensions` | required | Non-null balance coordinates. At least one is required. |
| `resources` | required | Additive signed values. At least one is required. |
| `migration.enabled` | `true` | Generate the concrete register migration. |
| `migration.name` | `create_<tableName>_register` | Stable migration name. |

### Dimensions

Dimensions support these version 1 types:

| `type` | Default PostgreSQL type | Generated Go type |
|---|---|---|
| `int` | `integer` | `int` |
| `bigint` | `bigint` | `int64` |
| `string` | `text` | `string` |

A string dimension may specify `sqlType: varchar(length)`. A dimension may declare the same `references` properties as an object field: `schema`, `table`, `column`, `onDelete`, and `onUpdate`.

Dimensions are deliberately non-null in version 1. Nullable values make composite keys, equality joins, filters, and aggregate deletion semantics ambiguous.

### Resources

Resources support `numeric`, `int`, and `bigint`. Numeric resources may specify `numeric(precision, scale)`, for example `numeric(19, 4)`. The generated Go mappings are `float64`, `int`, and `int64`, respectively, matching the ordinary object generator. Every action must have at least one non-zero resource.

Applications that require an exact decimal Go type should keep the PostgreSQL `numeric` resource but call the generated SQL functions from a hand-written decimal-aware repository until custom register Go types/imports are supported.

Resources are signed and additive. The register does not assign business meaning to the sign: a recorder decides whether a receipt is positive, a consumption is negative, or a transfer produces both movements.

## Common runtime repository

The canonical common SQL is stored and versioned inside Codegen:

```text
internal/codegen/templates/register/runtime/v1/
	bootstrap.up.sql.tmpl
	bootstrap.down.sql.tmpl
```

These templates are embedded into the Codegen executable. A consuming application does not copy a template repository and has no runtime dependency on Codegen.

When the first register is generated, Codegen creates the common migration before any concrete register migration:

```text
000010_register_common_v1.up.sql
000010_register_common_v1.down.sql
000011_create_materials_register.up.sql
000011_create_materials_register.down.sql
```

The common migration contains:

- `register_settings`;
- PostgreSQL timezone validation;
- `register_runtime_version()`;
- `register_business_timezone()`;
- `register_month_start()`;
- `register_month_start_at()`.

The generated migration contains the marker `codegen:register-runtime version=1`. Runtime templates are immutable after release. A future fix must use a new runtime version and a new upgrade migration rather than editing an applied version 1 migration.

An existing common-runtime pair is skipped by `generate` unless migration overwrite is explicitly enabled. `check` renders the frozen versioned runtime template and detects a missing or modified pair.

## Generated concrete register SQL

For `Materials`, Codegen generates:

- `ra_materials`: authoritative immutable action ledger;
- `rg_materials_period`: monthly net deltas, not cumulative month-end balances;
- `rg_materials_current`: fast current posted balances;
- recorder, effective-time, balance, and period indexes;
- foreign keys for declared dimensions.

It also generates typed PostgreSQL functions:

- `ra_materials_add_act(...)`;
- `ra_materials_remove_acts(recorder_type, recorder_id)`;
- `rg_materials_apply_delta(...)`;
- `rg_materials_rebuild()`;
- `rg_materials_balance(dimension_filters...)`;
- `rg_materials_balance(effective_at, dimension_filters...)`;
- `rg_materials_summary(from, to, dimension_filters...)`.

The action table rejects `UPDATE`. Insert and delete triggers apply or reverse every resource in both aggregate tables. Aggregate rows are removed when all resources become zero.

The historical balance function returns the balance immediately before `effective_at`. The summary uses the half-open interval `[from, to)` and returns opening, increase, decrease, and closing values for every resource. Use the next boundary for inclusive calendar reporting, for example the next local day at midnight as `to`.

## Generated Go repository

Backend generation creates:

```text
internal/registers/materials.gen.go
internal/registers/registry.gen.go
```

The concrete file contains typed action, filter, balance, and summary structures plus helpers to add/remove actions, query balances, summarize a period, and rebuild the register.

The registry contains metadata and common operations:

- `Definitions()`;
- `IsGenerated()`;
- `LockRecorder()`;
- `Rebuild()`;
- `RebuildAll()`.

`LockRecorder` includes both the register name and recorder type in its advisory-lock namespace. Unrelated registers therefore do not contend merely because their documents use the same type and ID.

Generated Go helpers use `ds.Querier`, so they work with either a transaction or another compatible primary connection. Document posting should pass the transaction.

## Recorder posting

Recorder posting remains application-owned in version 1. A document service should:

1. Start or join the primary transaction.
2. Lock the register/recorder pair.
3. Save and reconcile the document header and lines.
4. Remove the recorder's previous actions.
5. Add replacement actions calculated from the saved document.
6. Commit atomically.

For the materials register:

| Recorder | Movements |
|---|---|
| Receipt | Positive quantity at the receipt construction site |
| Consumption | Negative quantity at the consumption construction site |
| Transfer | Negative at the source and positive at the destination |

Use `service.manualMethods` for document `create`, `update`, and `delete` operations that post registers. Keep the generated `.gen.go` register repository and document service shell; place recorder-specific mapping in a hand-written extension such as `materialReceipt_custom.go` or `material_register.go`.

Do not expose action or aggregate relations through generic CRUD routes. Specialized reports and Vue layouts remain application-owned and consume the generated balance or summary functions.

## Timezone changes and rebuilds

The business timezone determines the monthly bucket for every action. Never rewrite the bootstrap migration to change it in an existing application.

Instead:

1. Add a new application migration that updates `register_settings`.
2. In the same maintenance migration, call every generated `rg_*_rebuild()` function before committing.
3. Use `registers.RebuildAll` for an application-level maintenance command or explicit recovery operation.

The immutable action ledgers remain the source of truth, so rebuilding does not lose business movements.

## Version 1 boundaries

Version 1 deliberately does not generate:

- document recorders or document-table triggers;
- HTTP CRUD for action/aggregate relations;
- specialized report hierarchies or Vue views;
- nullable dimensions;
- daily or yearly period aggregation;
- migration diffs after a register schema has been applied.

Changing a concrete register after its initial migration requires a new application migration. `migrations.overwrite` is only appropriate while the generated migration is still an unapplied draft.

Concrete register migrations are create-once scaffolds. After their pair exists, `generate` keeps it unchanged and `check` verifies the pair exists without comparing it to the current register YAML. This prevents an applied historical migration from becoming a regenerable desired-state file. The application remains responsible for adding an explicit follow-up migration whenever a register schema changes.
