# Changelog

- Distinguish inline date-only and date+time columns: `date` generates `dataType: "date"`, while `datetime`/`timestamptz` generate `dataType: "datetime"`.
- Preserve safe date/time defaults in inline drafts, including `now()`/`CURRENT_*` as `new Date()`.
- Fix generated inline collection labels so explicit list-column labels are not shadowed by disabled/default form labels.
- Disable page-form scaffolding by default for `frontend.list.editMode: inline`; it can still be explicitly enabled.
- Add `frontend.list.columns[].reference` metadata for generated reference-backed collection columns, including automatic named imports and `referenceField`.
- Add `frontend.list.columns[].sortField` passthrough for projected/server-side sort expressions.
- Enable generated inline editing for date, datetime, and timestamptz columns using their date-only/date+time metadata.
- Generate `never` update models for read-only frontend collections so their
  types match the generated CRUD API contract.
- Add independent `httpRoutes.enabled` and `httpRoutes.manualMethods` ownership
  for generated backend endpoints and binders.
- Derive route registration, helper imports, and manual-collision checks from the
  routes that remain generator-owned while preserving CRUD services,
  permissions, tests, and frontend contracts.
- Add independent YAML `detail` projections for Go detail services and models.
- Generate detail TypeScript types, Valibot schemas, API converters, and matching
  collection/edit-page detail types; preserve base mutation models.
- Validate detail fields, keys, imports, and generated-name collisions.
- Document configuration and add `examples/products_detail.yaml`.


## Unreleased

- Fix access write-target validation to accept unquoted case variants, detect
  case-folded duplicate targets, and normalize columns before modelbind metadata
  checks and registration without weakening non-writable-field protection.
- Fix protected manual service/HTTP extensions being rejected solely because
  their filenames match an object; retain declaration/registration collisions
  and the existing behavior for unprotected objects.
- Preserve omitted versus explicitly empty access maps during YAML marshaling,
  fixing deterministic regeneration/removal fixtures without allowing writes on
  read-only representations.
- Add access regression coverage for strict YAML round trips, empty mappings,
  case-normalized metadata checks, manual extension generation/check, and actual
  declaration/route collisions.

- Add opt-in object access-resource identities and explicit base/list/detail/key
  policy bindings, including hidden columns and metadata-declared projection aliases.
- Generate value/pointer-safe model markers and one error-returning
  `services.RegisterAccessModels` entry point without business policies or controllers.
- Prepare protected path-key update bodies before validation through the existing
  webapp public API; preserve original partial-update presence and separate keys.
- Register typed JSON decoders selected from actual generated/handwritten method
  signatures; bridge value signatures through pointer-based preparation APIs.
- Add explicit extra-method operations and handwritten decoder integration,
  fail-closed ownership checks, and manifest-tracked stale access artifact cleanup.
- Preserve unprotected output, enums, frontend ownership/output, CRUD switches,
  permissions, service factories, and access-aware notification behavior.
- Add parser/rendering/ownership tests, standard-library syntax checks, and a native
  generated-application fixture targeting the supplied real policy-enabled libraries.
- Document access YAML, dependency snapshots/local replacements, trusted startup,
  transport formats, manual method boundaries, and verification requirements.

- Generate shared Go enum declarations, typed constants, values helpers, `IsValid`,
  and `String` methods from top-level enum definitions.
- Register generated enum values with modelbind during models-package startup,
  merging unrelated registrations and rejecting conflicting entries.
- Add `enums[].backend.generation` and `register` ownership controls; preserve
  manual declarations and legacy backend enum fields without definitions.
- Add constant-name overrides and backend declaration/method collision checks.
- Track backend enum artifacts in `.codegen/backend-enums.json` for nondestructive
  checks and guarded stale-file cleanup.
- Add opt-in shared initial PostgreSQL enum migrations, dependency-order checks,
  immutable history, and restrictive down migrations; do not alter existing
  database columns or infer destructive enum evolution.
- Add backend enum regression/runtime fixtures and ownership/integration docs.

- Add shared top-level enum definitions, metadata inference, cross-schema conflict
  validation, and explicit `frontendDefault` values for typed inline drafts.
- Generate managed TypeScript enum values/literal unions, locale-aware Valibot
  picklists, typed base/detail/list models, and localized enum labels.
- Generate native enum collection columns with `enumOptions`, nullability-aware
  clear behavior, and preserved format/search/editor overrides.
- Scaffold enum form Select inputs with unselected drafts when no default is set;
  preserve developer-owned Vue files and generated contract boundaries.
- Track generated enum ownership for `check` and scoped stale-file cleanup;
  leave manual translations and application-owned artifacts alone; backend enum
  generation is controlled independently as documented above.
- Validate the minimum enum collection-library version (0.1.10) only for affected
  frontends, without rewriting package manifests or lockfiles.
- Preserve legacy backend-only enums and existing reference, label, datetime,
  projection, and non-enum output behavior.
- Add enum regression tests, a MaterialStatus example, and frontend enum docs.

- added monthly accumulation-register schemas with typed dimensions and additive resources;
- added the embedded, versioned register common-runtime repository and automatic bootstrap migration;
- generated immutable action ledgers, monthly/current aggregates, balance, summary, and rebuild functions;
- generated typed Go register repositories, namespaced recorder locking, and rebuild registry operations;
- documented register configuration, schema ownership, migration lifecycle, recorder posting, and timezone rebuilds;
- added `service.manualMethods` for per-operation hand-written Go service implementations while retaining generated routes, permissions, models, and frontend contracts;
- added validation for missing manually owned service methods and generated/manual receiver-method collisions;
- added `frontend.list.editMode: inline` for generated Vue collection pages;
- added `frontend.list.columns[].editable` overrides;
- generated native inline create drafts and typed create-model mapping;
- enabled inline create/edit commands without requiring page-form routes;
- added validation for unsupported inline editors and create fields without safe defaults;
- documented inline collection editing and added a complete inline example.

## v0.1.0

Initial standalone release.

- extracted the generator into its own Go module;
- embedded default Go, SQL, and Vue templates into the generator binary;
- added committed `codegen.yaml` project configuration support;
- retained `CODEGEN_*` environment overrides for compatibility;
- added `generate`, `check`, and `validate` commands;
- resolved relative paths from the configuration file directory;
- retained backend/frontend manual-collision guards and existing schema semantics;
- documented Go 1.24+ tool dependency and Go workspace workflows.
