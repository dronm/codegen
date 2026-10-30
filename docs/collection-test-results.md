# Collection integration verification

Verified on 2026-10-30 using Go 1.25.0, Node 24.22.1, TypeScript 6.0.3, and the actual
`@katren/vue-collection-lib` v0.1.15 source. Backend integration used the updated
grouped-capable `webapp` and `modelbind` source trees, with a local DS module.
No application dependency files are changed by the generator.

## Results

| Check | Result |
|---|---|
| Full generator suite, including dependency-backed fixtures | 137 top-level tests passed; 326 tests and subtests passed; no failed or skipped tests |
| `go vet ./...` | Passed |
| Generated grouped Go application: compilation, runtime tests, and vet | Passed |
| Existing generated access-policy application: compilation, runtime tests, and vet | Passed |
| Generated collection, API, model, and DTO TypeScript against v0.1.15 | Passed |
| Executable complete `CollectionParams` / `where` forwarding assertion | Passed |
| Original-release output comparison | All 16 original generated files byte-identical over two generations; subsequent `check` passed |
| Paginated and infinite-scrolling examples | CLI validation passed |

The grouped endpoint fixture exercises the generated HTTP binder, generated
service, and actual backend collection helper. At the database I/O seam it
checks nested continuation SQL and bound values in both row and aggregate
queries. The first request returns `[5, 4]`; the continuation returns `[3, 2]`.
It also verifies legacy `from`/`count` behavior and rejects malformed `where`
before SQL execution. This checks the real query-building path; it does not
require or claim a live PostgreSQL test database.

Regression tests cover omitted/disabled options, backend-only grouping,
automatic grouping and conflicts, explicit zero settings, invalid numeric
settings and unknown properties, non-`id` JSON keys, custom projections,
missing/optional/incompatible cursor fields, composite-key page restrictions,
independent route/service ownership, handwritten method signatures and aliases,
customized Vue scaffold preservation, dependency policies, and repeat
generation followed by `check`.

## Reproduce

Run `go test ./...` and `go vet ./...`. To include the real-library fixtures,
set the source-directory environment variables described in
[the integration guide](collection-scrolling.md#generator-integration-checks).
The existing access-policy fixture additionally uses
`CODEGEN_ACCESS_WEBAPP_DIR`, `CODEGEN_ACCESS_MODELBIND_DIR`, and optionally
`CODEGEN_ACCESS_DS_DIR` / `CODEGEN_ACCESS_SESSION_DIR`.

Suggested Git commit message:

```text
feat(codegen): add opt-in grouped filters and infinite collections
```
