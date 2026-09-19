# Frontend enums

Codegen can use a shared enum definition to generate TypeScript literal unions,
Valibot picklist schemas, localized collection options, and valid inline drafts.
This does not transfer ownership of PostgreSQL enum types or Go declarations to
Codegen.

## Define the enum and reference it

Declare enums at the top level of an ordinary object YAML file:

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

list:
  model: MaterialStatusList
  table:
    schema: public
    name: material_statuses_list
  fields:
    # Include the remaining list fields and keys as usual.
    - name: status
      type: enum
      enum: material_status_type
      required: true
```

This is an excerpt, not a complete object. A runnable schema example is
[`examples/material_status_enums.yaml`](../examples/material_status_enums.yaml).

`name` remains the backend modelbind validation registry name. `goType` and
`frontendName` are required type identifiers. `sqlType` is an optional SQL type
name or schema-qualified type name. When supplied, it is inherited by matching
base, list, and detail fields. Explicit field `goType`/`sqlType` values are
accepted when consistent with the definition and rejected when conflicting.
For nullable fields, Codegen preserves pointer/nullable handling.

All active object files are loaded before enum references are resolved. An
object may reference an enum declared in another loaded object. Identical shared
definitions are accepted and generated once. Duplicate names within one object,
or conflicting metadata, value order, or labels across objects, are errors.
A file containing only `enums` is not a standalone schema format: enum definitions
currently live in normal object specifications.

Enum values retain their declared order. Empty values, duplicates, conflicting
generated names, and conflicting model/locale namespaces are rejected. Defined
enum fields in matching base/list/detail projections must preserve their enum,
nullability, and property optionality rather than downgrading to `text`.

## Defaults and nullability

`frontendDefault` is a string literal for the frontend enum, not a TypeScript
expression and not a PostgreSQL default. For example:

```yaml
- name: status
  type: enum
  enum: material_status_type
  required: true
  frontendDefault: at_work
```

Both the inline row factory and create payload use the typed value `"at_work"`.
List/detail enum projections inherit the base field's default unless they have
an explicit default of their own. The field-level `frontendDefault` property
currently applies to enums only; ordinary form-specific `default` behavior and
existing non-enum field defaults are unchanged.

A recognized SQL literal is also usable as an explicit enum default:

```yaml
default: "'at_work'::public.material_status_types"
```

This additionally defines the database default in a new migration draft.
`frontendDefault` takes precedence over a SQL literal for frontend creation. Each
selected frontend value must belong to the enum. Arbitrary SQL expressions such
as `compute_status()` are not evaluated; supply `frontendDefault` separately.

For inline creation, every enum property that the generated draft must contain
needs a valid explicit default unless its model type allows null or omission.
Codegen never substitutes `""` or silently chooses the first enum member. This
also applies to non-nullable enum fields hidden from the visible columns because
the typed row/create model still needs a valid value. Nullable enum fields without
a default start at `null`; optional properties may start at `undefined`.

Ordinary form scaffolds may start with an unselected optional draft. They use a
PrimeVue `Select`, emit the draft to the shared edit-page controller, and rely on
the generated picklist schema for submission validation. Custom enum components
remain possible; forcing a typed enum into a built-in text/number/checkbox editor
is rejected. Existing developer-owned Vue forms are not rewritten: update their
editors manually after converting a formerly plain string field to an enum.

## Generated types and schemas

For `frontendName: MaterialStatusType`, Codegen creates managed files:

```text
src/types/enums/materialStatusType.gen.ts
src/schemas/enums/materialStatusType.gen.ts
```

The type module exports an ordered `MATERIAL_STATUS_TYPE_VALUES` readonly tuple,
the `MaterialStatusType` string-literal union, and a
`MaterialStatusTypeOption` interface with typed `value` and string `label` fields.
It does not emit a TypeScript `enum` declaration.

Base/detail/list DTO and domain models import this union automatically. Create
and update types retain the same enum through their existing generated type
relationships. Nullable and optional wrappers are preserved. Multiple uses of
an enum share one import.

The schema module exports `createMaterialStatusTypeSchemas(t)` and the default
`MaterialStatusTypeSchema`. Model schema factories initialize the locale-aware
factory once and use its `v.picklist` schema instead of generic string validation.
Generic string validators are not retained merely because an enum is present.
An explicit custom Valibot expression may wrap the generated enum schema, but
must not silently replace enum membership validation with a string schema.

## Collection columns

```yaml
frontend:
  scaffold: true
  form:
    enabled: false
  list:
    editMode: inline
    columns:
      - field: status
        label: Статус
        editable: true
        width: 18rem
```

The managed collection imports the values tuple and generates:

```ts
{
	field: "status",
	headerKey: "MaterialStatus.fields.status",
	sortable: true,
	editable: true,
	dataType: "enum",
	enumOptions: MATERIAL_STATUS_TYPE_VALUES.map((value) => ({
		value,
		labelKey: `MaterialStatusType.${value}`,
	})),
	editorProps: {
		showClear: false,
	},
	width: "18rem",
}
```

The reusable library owns enum display, filtering, and editing. No custom editor
slot, generated enum component, or replacement for `CollectionListPage` is needed.
The inferred `dataType` can be stated explicitly as `enum`, but a conflicting
string/reference datatype is rejected.

Required/non-nullable enum columns default to `showClear: false`; nullable enum
columns default to `true`. Optional-but-non-nullable properties still default to
`false`, since clearing a selector produces null rather than omitting a property.
Explicit `editorProps.showClear` overrides this default; the application remains
responsible for matching that choice to its validation contract.

Existing width, sortField, sortable, align, and editable configuration is retained.
Additional column overrides are available:

```yaml
- field: status
  format: "(value) => String(value)"
  searchable: true
  searchField: status
  searchDataType: enum
  searchOperations: [exact]
  editorProps:
    showClear: false
    filter: true
```

`format` is a trusted TypeScript expression, not a string formatter name to be
looked up dynamically. It is emitted verbatim; any external identifiers must be
available in the generated module. `editorProps` accepts JSON-compatible values.
Explicit properties override inferred defaults. Without overrides, enum-specific
search and localized display are delegated to the collection library.

## Localization

Generated `src/locales/ru.gen.json` includes:

```json
{
	"MaterialStatusType": {
		"at_work": "В работе",
		"on_maintenance": "На обслуживании"
	}
}
```

`frontendName` is the namespace and each enum value is its translation key. The
ordinary field label remains under `MaterialStatus.fields.status`. The generated
locale file is separate from manual translations. Keep the existing application
i18n merge/import; Codegen does not edit manually maintained locale files.

## Collection-library dependency

Enum columns require the enum-capable `@katren/vue-collection-lib` API introduced
in `0.1.10`. For an npm dependency, declare for example:

```json
{
	"dependencies": {
		"@katren/vue-collection-lib": "^0.1.10"
	}
}
```

Codegen does not own an application's `package.json` or lockfile, and therefore
checks the requirement rather than rewriting either file. The minimum is
centralized in `MinEnumCollectionLibraryVersion`. This check applies only when
frontend generation includes enum collection columns; non-enum projects and
backend-only generation do not acquire a new dependency requirement.

The check accepts stable exact, caret, tilde, and simple `>=` versions whose lower
bound is at least `0.1.10`. Complex npm ranges, wildcard ranges, tags such as
`latest`, and prereleases do not establish the minimum for this check. `file:` and
`link:` directories are checked through their local package metadata; `workspace:`
references can be verified using the installed package or an explicit compatible
version. If an installed package is present but older than the minimum, update it
before generating. A version declaration alone cannot verify an unpublished
package's actual feature implementation; use a library build with enum support.

## Ownership, cleanup, and check

Commit `<frontend>/.codegen/frontend-enums.json` with generated source. It records
only the enum artifacts owned by Codegen. Referenced enums are generated once,
even when used by several objects. When the last reference is removed or its
frontend name changes, `generate` removes the old tracked enum files.

`check` verifies enum file contents and the ownership manifest. It reports missing,
changed, or stale outputs but never deletes them. Untracked manual enum files are
not swept. A target missing the generated enum ownership header, or a non-regular
target, is not overwritten/deleted. Files retaining the generated header remain
managed; do not edit them manually. Custom enum templates must preserve the
standard generated-file and `Frontend enum:` header lines.

This cleanup is intentionally narrower than general model cleanup: it does not
delete historical migrations, ordinary stale model files, manual Vue scaffolds,
or manual translations. Existing generated-file collision safeguards remain.

## Backend compatibility and database ownership

Legacy backend-only fields can still supply `type: enum`, an explicit `goType`,
and a modelbind `enum` registry name without a new definition. When frontend
artifacts must be generated for such a field, a definition with values is required:

```text
frontend enum field "status" references enum "material_status_type", but no enum values are defined
```

Applications retain responsibility for the Go enum declaration, modelbind enum
registration, and PostgreSQL `CREATE TYPE`/`ALTER TYPE`/`DROP TYPE` migrations.
Codegen continues to render Go field types, `enum:"..."` tags, and SQL column
types, but does not create or rewrite shared enum types or registration functions.
Changing an existing database column from text to an enum still needs a separate,
reviewed migration; adding frontend enum metadata does not migrate database data.

## Verification

```bash
go test ./...
go vet ./...
go tool codegen validate
go tool codegen generate
go tool codegen check
```

The real YAML/generation integration regression is
`TestLoadObjectsParsesFrontendEnums`. `TestEnum*` tests cover resolution, invalid
metadata/defaults, typed rendering, nullability, columns, localization, dependency
requirements, import deduplication, and safe cleanup. Run the consuming project's
normal frontend typecheck against its actual enum-capable library version.
