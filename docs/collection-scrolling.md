# Grouped collection filters and infinite scrolling

Existing schemas keep their collection endpoint signatures and paginated Vue
behavior. Both features are opt-in. They use the existing collection URL,
`CollectionListPage`, managed collection definition, and application CRUD API.

Start with [the paginated example](../examples/collection_paginated.yaml) or
[the infinite scrolling example](../examples/collection_infinite.yaml). The
latter includes a custom list projection and a key whose JSON name is
`entry_code`, rather than `id`.

## Schema configuration

These settings belong to each object schema, rather than `codegen.yaml`:

```yaml
collection:
  groupedFilters: true

frontend:
  scaffold: true
  list:
    infiniteScroll:
      enabled: true
      chunkSize: 100
      prefetchRows: 30
      maxCachedChunks: 5
      scrollHeight: 32rem
      itemSize: 40
```

| Property | Default | Meaning and validation |
|---|---|---|
| `collection.groupedFilters` | `false`; automatically `true` for infinite scrolling on generated list routes | Use the grouped backend collection contract. Explicit `false` conflicts with enabled infinite scrolling. |
| `frontend.list.infiniteScroll.enabled` | `false` | Enable the library's infinite scrolling mode; requires `crud.list`, `frontend.scaffold`, and an enabled frontend list. |
| `chunkSize` | `100` | Positive integer: rows requested per chunk. |
| `prefetchRows` | `30` | Nonnegative integer: lookahead distance; explicit `0` disables lookahead prefetching. |
| `maxCachedChunks` | `5` | Nonnegative integer: retention limit; explicit `0` retains unlimited chunks. |
| `scrollHeight` | `32rem` | CSS viewport height passed to the collection library; an empty string uses the default. |
| `itemSize` | `40` | Positive finite number: estimated row height in pixels. |

Numeric options are validated even when `enabled: false`; errors identify the
schema file and property. Unknown properties remain errors. Omitting
`infiniteScroll`, or setting `enabled: false`, emits no `infiniteScroll` property
and retains the existing `frontend.list.pageSize` behavior. The paginated default
remains 30. The library uses `chunkSize` when infinite scrolling is enabled.

`collection.groupedFilters: true` works without frontend generation. A schema
with enabled infinite scrolling can also be generated in a backend-only run;
the global `frontend.enabled` setting still controls whether frontend files are
written. Enabling either feature requires `crud.list: true`.

## Generated backend contract

An opted-in generator-owned endpoint changes all three parts together:

| Part | Legacy output | Grouped output |
|---|---|---|
| Route binder | `webapp.CollectionParamsBinder()` | `webapp.CollectionParamsWithWhereBinder()` |
| `List` argument | `modelbind.CollectionParams` | `modelbind.CollectionParamsWithWhere` |
| Fetch helper | `webapp.FetchCollectionModel(...)` | `webapp.FetchCollectionModelWhere(...)` |

The grouped endpoint still accepts ordinary `filter`, `sorter`, `from`, and
`count` requests. It additionally accepts grouped `where` conditions, including
the continuation boundary constructed by the Vue library. The URL, GET method,
permission, session checks, notifications, list model, and response envelope
remain the same. Create, detail, update, and delete retain their existing
generation.

Update the application to `webapp` and `modelbind` revisions exposing the APIs
above. No release numbers are assumed, and Codegen does not edit the
application's `go.mod` or other dependency files. A dependency update alone
does not change an old generated endpoint's binder, signature, or fetch call;
regenerate the opted-in object too.

For application-owned connection handling, use
`webapp.FetchCollectionModelWhereWithConn(...)` with the matching grouped
parameters. For custom SQL, use `pg.NewPgGroupedSelect` and
`modelbind.BindCollectionSelectModelWithWhere`, then build SQL through the
grouped wrapper. Keep trusted authorization restrictions and aggregate
filtering in the same query path; a client continuation condition must narrow
the authorized result. Follow the current library signatures when adapting
custom code. The built-in collection template uses the fetch helper and does
not generate a separate custom-SQL collection path.

## Keys, projections, and ordering

The managed collection definition receives the existing library configuration:

```ts
infiniteScroll: {
	keyFields: ["entry_code"],
	chunkSize: 100,
	prefetchRows: 30,
	maxCachedChunks: 5,
	scrollHeight: "32rem",
	itemSize: 40,
},
```

`keyFields` comes from the real schema keys and their backend JSON/filter names.
It is independent of the route parameter name and is never hard-coded to `id`.
Each cursor key must be present as a required, non-null JSON-visible value in
the list projection, its DTO/model types, conversion, and validation schemas.
Keep custom list fields compatible with the base keys. The generated list
schema and converter must retain those values, even when a key has no visible
column in the page.

Custom `valibotDto`, `valibotModel`, type, and conversion expressions remain
trusted application code. Codegen checks detectable incompatible cursor
representations, but cannot prove that an arbitrary custom validator or
transform preserves the exact key value. Verify that runtime validation retains
every cursor key and that conversion neither removes nor changes it.

Generated collection pages still require a single key. Backend composite-key
APIs remain supported, but deriving multiple key names does not enable a
generated composite-key collection page; keep such pages application-owned.

Existing column `sortField` mappings are preserved. Cursor values must represent
the values actually ordered by the backend. For derived/reference columns,
verify that the value used by the library matches the server-side expression,
including null ordering and normalization. For timestamps, JavaScript `Date`
retains milliseconds, while PostgreSQL timestamps can retain finer precision.
Preserve an exact sortable representation or deliberately align the backend
ordering precision before enabling that sort for scrolling. A unique key
tie-breaker does not recover precision already lost from another sort value.

Codegen supplies metadata only. The Vue library owns cursor construction,
chunk buffering, virtual rendering, prefetching, and keyboard navigation.

## Frontend dependency and transport

Affected frontends require `@katren/vue-collection-lib` **0.1.15 or newer**.
Codegen reuses its collection dependency checks, including local `file:`,
`link:`, and workspace dependencies with verifiable compatible metadata. It
does not rewrite `package.json` or lockfiles. Backend-only runs do not require
an installed frontend dependency.

Generated API modules use the application's `@/api/createCrudApi`. Its list
method must accept and forward the complete library `CollectionParams` to the
HTTP transport, including `where`. Inspect that application-owned helper before
enabling scrolling. A helper that reconstructs only `filter`, `sorter`, `from`,
and `count` drops the continuation boundary and repeatedly requests the first
chunk. Preserve `where` through any query serialization and proxy layer, along
with all existing collection parameters. Keep the helper's existing response
conversion and envelope handling.

Do not add a new endpoint, cursor token, or an application-owned scrolling loop
to compensate for missing `where` forwarding. Test the helper with a grouped
request and verify that the next response is beyond the previous chunk's
boundary.

## Migration and manual ownership

For an existing generated project:

1. Update the application's backend libraries and frontend dependency as above.
2. Update the application-owned `createCrudApi` helper to preserve complete
   collection parameters, including `where`.
3. Enable the per-object schema settings. Run `codegen validate`, then
   `codegen generate`; review the binder, method argument, and fetch helper
   together. Run the application's Go tests and frontend type-check/build.
4. Run `codegen generate` again and `codegen check`; the second generation
   should be stable. Exercise both an ordinary paginated request and a grouped
   continuation request against the same collection endpoint.

`service.manualMethods` and `httpRoutes.manualMethods` remain independent:

| Ownership of list | Required migration |
|---|---|
| Generated route and generated service | Codegen updates the binder, `List` signature, and fetch helper together. |
| Generated route, manual service | Migrate the manual `List` argument to `modelbind.CollectionParamsWithWhere` and use the grouped fetch/query path. Codegen diagnoses detectable incompatible handwritten signatures. |
| Manual route, generated service | Declare `collection.groupedFilters: true` explicitly, migrate the application binder, and connect it to the generated grouped service. |
| Manual route and manual service | Explicitly declare grouped capability and migrate the route binder, service argument, and fetch/query path in application code. |

A disabled generic route set (`httpRoutes.enabled: false`) also leaves route
implementation with the application. Enabled infinite scrolling requires an
explicit `collection.groupedFilters: true` declaration for application-owned
list routes; automatic opt-in cannot certify manual code. The declaration is a
capability promise, and Codegen does not rewrite or prove custom route/query
logic. An explicit `false` is a validation error with infinite scrolling.

Place handwritten implementations in extension files and retain normal
collision checks. No manual methods, routes, or existing Vue scaffolds are
overwritten. Options live in the managed `src/collections/<object>.gen.ts`, so
an existing list scaffold using `CollectionListPage` and that definition picks
them up. A custom scaffold that copies old options or bypasses the managed
definition needs an application edit. `generate`, `check`, and `validate` retain
their existing ownership semantics.

## Custom template overrides

Projects using `templateDir` retain their overrides. Compare these overrides
with the updated built-ins before enabling the feature:

- `go/httpapi.go.tmpl`: choose the grouped binder from `ObjectView.GroupedFilters`.
- `go/service.go.tmpl`: choose the matching argument and grouped fetch helper;
  custom connection/SQL paths need the grouped APIs described above.
- `vue/collection.ts.tmpl`: emit `Frontend.List.InfiniteScroll` in the managed
  definition while preserving pagination when it is absent.
- `vue/api.ts.tmpl`: preserve complete `CollectionParams` forwarding and the
  application helper contract, including `where`.

Projection type/schema overrides must retain required cursor fields and their
DTO conversions. Custom list-page overrides should consume the managed
definition through `CollectionListPage`. Codegen cannot infer that an arbitrary
override implements grouped filtering merely because its dependencies exist.

## Generator integration checks

Run the ordinary suite with `go test ./...`. Additional executable fixtures use
real application dependencies when their local source paths are supplied:

```bash
CODEGEN_COLLECTION_WEBAPP_DIR=/path/to/webapp \
CODEGEN_COLLECTION_MODELBIND_DIR=/path/to/modelbind \
go test ./internal/codegen -run TestCollectionGeneratedApplicationIntegration -count=1 -v

CODEGEN_COLLECTION_LIB_ROOT=/path/to/vue-collection-lib \
go test ./internal/codegen -run TestInfiniteGeneratedTypeScriptAndWhereForwarding -count=1 -v
```

The backend fixture requires Go 1.25 or newer. It compiles generated routes,
service, and projection against the grouped-capable libraries and exercises
continuation filtering through the HTTP and SQL path. It also verifies repeated
generation and subsequent `check`, then runs generated-application tests and
`go vet`. Optional `CODEGEN_COLLECTION_DS_DIR` and
`CODEGEN_COLLECTION_SESSION_DIR` select local supporting modules. Without
replacements, Go resolves those dependencies normally.

The frontend fixture needs a collection-library checkout with installed Node
dependencies. It type-checks generated TypeScript and runs a transport assertion
that complete grouped parameters, including `where`, reach the request. These
dependency-backed tests skip when their required source paths are absent;
schema, rendering, ownership, and compatibility tests still run normally.
