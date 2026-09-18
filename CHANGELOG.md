# Changelog

## Unreleased

- Add `frontend.list.columns[].reference` metadata for generated reference-backed collection columns, including automatic named imports and `referenceField`.
- Add `frontend.list.columns[].sortField` passthrough for projected/server-side sort expressions.
- Enable generated inline editing for date, datetime, and timestamptz columns using `dataType: "date"`.
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
