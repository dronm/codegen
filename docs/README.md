# Codegen documentation

- [Configuration and CLI](configuration.md) — commands, `codegen.yaml`, environment overrides, paths, and embedded templates.
- [Consumer integration](integration.md) — Go tool dependency, Make targets, workspace development, registry hooks, and CI.
- [Object schema reference](schema-reference.md) — complete YAML object contract and generated backend/frontend shape.
- [Access policies](access-policies.md) — generated bindings, policy-aware transport inputs, application-owned policies, and local dependency integration.
- [Backend enums](backend-enums.md) — generated Go types/constants, modelbind registration, manual ownership, and opt-in PostgreSQL enum migrations.
- [Frontend enums](frontend-enums.md) — shared enum definitions, typed values and validation, inline defaults, localized columns, and ownership.
- [Accumulation registers](registers.md) — common runtime repository, register schemas, generated SQL/Go APIs, and recorder ownership.
- [Migrating from an embedded generator](migration-from-embedded.md) — one-time extraction procedure for existing applications.
- [Releasing](releasing.md) — tagging, consumer upgrades, and compatibility guidance.
