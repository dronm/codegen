# Changelog

## Unreleased

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
