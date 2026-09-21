# Access-policy integration

Codegen integrates the policy-enabled `modelbind` and `webapp` APIs. It generates
model identity, concrete SQL bindings, policy-aware HTTP input adapters where
needed, and typed JSON service decoders. It is **not** a second authorization
system. The application still authenticates users, resolves current assignments,
defines operation-specific policies, checks route/method permissions, and decides
which authenticated recipients may receive notifications.

## Required dependencies

This implementation targets the **source snapshots** supplied as
`modelbind_access_policies.zip` and `webapp_access_policies.zip`. No published
policy-enabled module version is assumed. Both snapshots declare Go **1.25.0**;
use that toolchain or a compatible newer toolchain for consuming applications and
native integration tests. The generator's existing Go directive is unchanged.

In the application, keep the existing module requirements and add local
replacements pointing to those policy-enabled source trees:

```bash
go mod edit -replace=github.com/dronm/modelbind=/absolute/path/to/modelbind
go mod edit -replace=github.com/dronm/webapp=/absolute/path/to/webapp
go mod edit -replace=github.com/dronm/codegen=/absolute/path/to/codegen
```

These commands assume the application already declares those dependencies and
the Codegen tool. Replacement paths in a dependency's own `go.mod` are not the
application's configuration; add any required local `ds/v4` or `session`
replacements in the application as well. Do not copy machine-specific paths into
shared production configuration. Neither application `go.mod` nor library source
is rewritten by generation. The verification report records the exact archive
hashes used for this patch.

## YAML: explicit, opt-in model protection

With no `access` section, or `access.enabled: false`, existing output and behavior
remain unchanged. To protect a generated object:

```yaml
access:
  enabled: true
  resource: Order
  bindings:
    model:
      rows: {owner: customer_id}
      writes: {owner: customer_id}
    list:
      rows: {owner: customer_id}
    detail:
      rows: {owner: customer_id}
    key:
      rows: {owner: customer_id}
```

`resource` defaults to the object's `name` **only when omitted**. An explicitly
empty string, whitespace-only string, YAML null, non-string, or control character
is rejected. Nonempty resource names are trimmed. Each representation shares that
one resource identity; policies are not registered independently for list/detail
Go type names.

`model`, `list`, `detail`, and `key` identify representations, not fixed names:
Codegen uses the actual base model, configured projection names, and generated
key type. The `Update...Request` wrapper has no `Relation()` and is not a database
representation. It is not marked or registered.

### Explicit binding rules

- Every distinct concrete database representation needs `rows`, including the
  generated key type. A base model used for create/update also needs `writes`.
  An explicitly configured extra insert/update service method makes the base
  representation writable too.
- Only the writable base representation accepts `writes`. List, detail, key,
  and read-only base representations reject it, even an empty write mapping.
- `{}` is accepted as an **explicit empty mapping**. It is not an allow policy.
  A policy requiring an unmapped logical field still fails in the libraries.
  YAML marshaling preserves this distinction: omitted/nil maps stay omitted,
  while deliberately empty maps remain `{}`. A marshal/load cycle must not add
  `writes: {}` to a read-only representation or erase a required empty mapping.
- A missing list/detail projection reuses the base concrete type. Omit its
  binding, or repeat exactly the base `rows` as an equality assertion, without
  `writes`. No second registration is emitted. A conflicting repeated binding
  is an error, not an override or implicit merge.
- Custom projections need their own explicit bindings even when they select
  from the base table. A duplicate concrete type declaration is rejected.

The left-hand side is the logical policy field; the right-hand side is the real
SQL column reference. `owner: customer_id` is supported. Logical fields must be
identifiers. Row targets accept unquoted identifier forms `column`,
`relation.column`, or `schema.relation.column`. SQL expressions, operators,
functions, JSON paths, comments, quoted identifiers, and arbitrary SQL are not
accepted. Write targets must be **unqualified** column names. They use unquoted
SQL identifier semantics: `CUSTOMER_ID` is accepted and normalized to
`customer_id` before metadata checks and generated registration. Logical policy
field names retain their spelling. Two logical write fields cannot map to the
same target, including case variants such as `customer_id` and `CUSTOMER_ID`.
Normalization does not bypass read-only/server-generated field checks or the
requirement to match the modelbind field ID.

For a known writable struct field, the target must agree with its modelbind field
ID and SQL column. A mapping to a server-generated, read-only, hidden-from-JSON,
or `srvCalc` field is rejected. Codegen does not remove `required` constraints or
reclassify an ownership column as server-generated to make validation pass.

### Columns omitted from projections and request models

A bound policy column does **not** need to be selected or exposed in a Go model.
For example, `OrderSummary` may return only `id` and `name` while a policy filters
its relation by `customer_id`. A write column absent from the input model can be
controlled by the policy-aware SQL builder rather than by a generated DTO field.

Codegen has no database catalog connection and cannot prove that an externally
managed view exposes a hidden column. An explicit mapping to such a column is an
**application declaration of that database fact**, not an inferred column. Check
that declaration against the actual table/view before deploying. Codegen checks
identifier syntax, known field metadata, and relation qualifiers; it never parses
view SQL to guess missing mappings. A typo in an external column can fail at SQL
execution; it never causes the binding to be silently removed.

### Relation aliases

For qualified mappings, declare the alias on the actual projection relation:

```yaml
list:
  model: OrderSummary
  table:
    schema: public
    name: orders
    alias: o
  fields:
    - name: id
      type: int
      primaryKey: true
    - name: name
      type: text

access:
  enabled: true
  bindings:
    model:
      rows: {owner: customer_id}
      writes: {owner: customer_id}
    list:
      rows: {owner: o.customer_id}
    key:
      rows: {owner: customer_id}
```

The generated list `Relation()` is `public.orders AS o`. This is a real alias in
the generated query, not authorization metadata guessing one. `detail.table.alias`
works likewise. Qualifiers not exposed by the representation are rejected; after
an alias is declared, use that alias rather than the hidden original relation
name. Writable `table.alias` is not supported. Alias-free existing YAML is unchanged.

The complete [Order example](../examples/access_order.yaml) includes separate
list/detail projections with hidden owner columns and distinct aliases.

## Generated output and application startup

For a protected Order with distinct list/detail models, Codegen adds:

```text
internal/models/order.access.gen.go
internal/httpapi/order.access.gen.go
internal/services/access_models.gen.go
.codegen/access.json
```

The models sidecar gives `Order`, `OrderSummary`, `OrderDetail`, and `OrderKey`
value-receiver `AccessResource()` methods. Compile-time assertions cover both
values and pointers. There is one marker and one registration per concrete type.
The ordinary `order.gen.go` models and service/HTTP files remain managed output.
A service sidecar `internal/services/order.access.gen.go` is added only when a
handwritten value-signature method needs a pointer-to-value decoder adapter.

The single application-facing entry point is:

```go
func RegisterAccessModels(registry *webapp.PolicyRegistry) error
```

It lives in the existing `internal/services` package, alongside generated service
registrations, and imports `internal/models`. Models do not import services.
Errors preserve their causes and identify the resource and model. Register once
per new registry; duplicate registration is an error. If a registration fails,
discard that partially configured registry instead of publishing it.

The access helper is emitted whenever protected backend models are generated,
even if `registries.enabled` is false: that existing setting disables ordinary
service/HTTP registry catalogs, not required access binding metadata. Startup
still must call the helper explicitly. There is no generated `init()` installing
an authorization controller or policy.

The full, **handwritten** startup example is
[`examples/access/bootstrap.go.txt`](../examples/access/bootstrap.go.txt). Copy it
to an application-owned package and replace `example.com/project` with the actual
application module. Its `IdentityStore` is an application interface to implement,
not a new requirement or API in `webapp`.

That example constructs a registry, calls `services.RegisterAccessModels`,
registers customer Read/Insert/Update/Delete policies with checked errors, and
assigns `app.Access` only after all registration succeeds. Principal resolution
revalidates the session and loads current roles and customer assignment from
trusted storage. It never trusts `customer_id` supplied in the URL/body or a
stale WebSocket login snapshot. Invalid/missing identity or assignment is denied.

Application-owned startup calls it alongside existing setup:

```go
if err := accesssetup.ConfigureAccess(app, identities); err != nil {
	return err
}
services.RegisterGeneratedServices()
```

Here `app` is the application's existing `*webapp.AppContext`; `identities`
implements that example's `IdentityStore`. Retain normal session middleware,
route/method permission checks, router setup, and connection management. Configure
access before accepting HTTP requests or dispatching WebSocket/event messages.
Add explicit policies for every other protected resource the application uses;
this example intentionally authorizes only Order. There is no admin bypass and
no automatically generated permissive fallback.

## HTTP preparation and field presence

For protected create routes, Codegen reuses `InsertInputBinder` or the appropriate
model-only counterpart. These patched standard binders already prepare access
input before validating required fields.

The supplied `UpdateByPathKeysBinder` instead validates the body too early.
Protected generated PATCH routes therefore use a generated adapter that:

1. Parses every path key with its configured path name and JSON/modelbind name.
2. Requires and validates all keys separately, including composite keys.
3. Decodes the typed request body and original `AbsentFields`.
4. Calls `webapp.PrepareAccessInput(r.Context(), input, access.Update)`.
5. Returns the exact typed `UpdateByKeysInput` expected by the service.

`PrepareAccessInput` constructs and validates effective input in the supplied
library; the adapter does not validate the body first or duplicate policy logic.
A trusted enforced insert value can satisfy an ordinary required field. Conflicts
and explicit null are still rejected by policy. Partial-update absence is not
converted to zero/null, and an absent enforced update field is not needlessly
added to the SET clause. Required path keys are not supplied by row policies.
Detail/delete key inputs are not passed through write-body preparation.

The supplied modelbind preparation/validation APIs require pointer models.
For known handwritten value-model/value-key signatures, adapters decode and
prepare pointers through the public APIs, then convert to the exact declared
value signature while retaining `AbsentFields`. Handwritten bodies must likewise
pass pointer models to CRUD helpers, for example:

```go
pointerInput := modelbind.ModelInput[*models.Order]{
	Model: &input.Model,
	AbsentFields: input.AbsentFields,
}
result, err := webapp.InsertModelInput(ctx, s.DB, pointerInput, nil)
```

Do not replace that `ctx` with `context.Background()`. Full-model inputs cannot
preserve field presence after it has been discarded; use `ModelInput` for PATCH.

## JSON WebSocket and local-event dispatch

Protected standard service registrations add typed `WithMethodInputDecoder`
options independently of HTTP binders:

```go
webapp.WithMethodInputDecoder(
	"Create",
	webapp.ModelInputJSONDecoder[*models.Order](true),
)
webapp.WithMethodInputDecoder(
	"Update",
	webapp.UpdateByKeysJSONDecoder[*models.OrderKey, *models.Order](),
)
```

The flat create/plain-ModelInput wire format is:

```json
{"name":"New order"}
```

A path-independent ByKeys update uses:

```json
{"keys":{"id":12},"input":{"name":"Changed order"}}
```

The first example is valid with the included Order schema and its handwritten
insert policy supplying `customer_id`; the generated ID is server-owned. Clients
must not serialize internal `ModelInput`, send `AbsentFields`, or submit a
`{"Model":...,"AbsentFields":...}` envelope. The server decoder derives presence
from the actual flat JSON object. Route/method permissions are separate from
input decoding. Transports must use the patched context-aware dispatch APIs with
an authorization-bearing context and a fresh access scope for each message.

Only existing protected write methods receive options; list/detail/delete keys
retain their existing handling. Existing factories, CRUD switches, permissions,
and `WithCRUDNotifications()` options are retained.

## Handwritten methods and explicit custom integration

`service.manualMethods` continues to suppress only method bodies. Codegen reads
handwritten declarations and retains generated infrastructure when their actual
signatures are supported. It handles import aliases, pointer/value models, plain
`ModelInput`, `UpdateByKeysInput`, and full models of the object's base type. It
never guesses the signature from the method name.

For protected objects, those extensions may be in `internal/services/order.go`
next to `order.gen.go`; manually owned routes may likewise be in
`internal/httpapi/order.go`. Their filename alone is not a collision. Codegen
checks declarations, method ownership, and route registrations and leaves the
handwritten files unchanged. Do **not** set `CODEGEN_ALLOW_MANUAL_COLLISIONS=true`
to enable this supported workflow: duplicate service types/factories, generated
methods/routes, model declarations, and access markers still produce errors.
Existing unprotected-object collision behavior is unchanged.

For an additional exported handwritten write method, specify its operation:

```yaml
access:
  enabled: true
  bindings:
    model:
      rows: {owner: customer_id}
      writes: {owner: customer_id}
    key:
      rows: {owner: customer_id}
  serviceInputs:
    Submit:
      operation: insert
    Change:
      operation: update
```

Those methods must exist in handwritten service files. Extra directly recognized
protected inputs without an explicit operation are rejected. The supported
signature is `(context.Context, input)`; no extra parameters are inferred.
Local named aliases, arbitrary wrapper structs, unrelated model types and other
custom signatures are not automatically unfolded. Their integration is explicit:

```yaml
access:
  enabled: true
  bindings:
    model:
      rows: {owner: customer_id}
      writes: {owner: customer_id}
    key:
      rows: {owner: customer_id}
  serviceInputs:
    Create:
      operation: insert
      decoder:
        name: DecodeOrderCreate
        import: example.com/project/internal/orderinput
httpRoutes:
  manualMethods:
    - create
```

`decoder.name` names a `webapp.ServiceInputDecoder` function/value, not a factory
expression; omit `import` for a function in the service package. Imported names
must be exported. A custom decoder must perform policy preparation before
validation and return the exact declared input type. The library checks that
returned type at dispatch. For an unsupported overall method signature, provide
handwritten service registration instead of enabling generated integration.

A handwritten Update accepting a plain `ModelInput` or full model needs a manual
HTTP update route because the default generated HTTP contract separates path
keys and body. Set `httpRoutes.manualMethods: [update]`; the supported typed JSON
decoder can still be generated. A fully custom DTO which Codegen cannot identify
as a protected model must be integrated manually even if it is not listed in
`serviceInputs`. Markers on other model types do not secure such a DTO or method.

## Ownership, determinism and removal

All added access files have managed headers. `.codegen/access.json` tracks paths,
kinds and content hashes, including ordinary backend files of protected objects.
Commit the manifest together with the generated source. It allows cleanup of
orphaned protected models/services/routes when a whole schema is removed.

`check` compares managed output and reports stale files without modifying them.
`generate` overwrites active managed artifacts, removes obsolete tracked files
only when their headers and last-generated hashes still match, and refuses
modified stale files, unsafe paths, symlinks, and handwritten targets. Removing
access from a still-present object regenerates its ordinary legacy backend files
and removes access-only sidecars/registrations. Removing the entire object also
removes its tracked ordinary backend outputs. Empty manifests are removed.

A previously generated access sidecar without a tracking manifest is not deleted
by guesswork. Preserve the manifest; when migrating a broken checkout, review and
remove obsolete untracked generated files explicitly. Codegen does not scan and
delete arbitrary `*.go` files.

Handwritten `AccessResource()` methods and manually declared model types produce
explicit collision errors, even with the general manual-collision escape hatch.
There is no safe automatic merge of handwritten authorization ownership. Resolve
ownership deliberately before enabling access. This baseline does not have an
object-wide `backend.generation` switch: `enums[].backend.generation` controls
**enum ownership** and is unchanged. Existing service/HTTP method ownership and
frontend managed/scaffold rules remain unchanged. No manual model, authorization
file, custom body, bootstrap, or frontend file is rewritten by access generation.

If the last protected object is disabled or removed, the generated binding helper
is removed too. Update the handwritten bootstrap call and obsolete business policy
registrations deliberately; Codegen does not edit application startup to hide
that change.

Disabling/removing access is an intentional removal of protection. Review it like
any other security change, regenerate successfully, and redeploy consistently.
The generator does not silently migrate an object from protected to unprotected.

## Security boundaries and notifications

The generated services keep using the supplied policy-aware CRUD helpers and
forward their incoming context. Missing access configuration, model registration,
principal, or applicable policy never turns a marked model into legacy unrestricted
access. Configuration failures and authorization denials keep the libraries'
existing distinct error behavior; there is no generated `AllowAll()` fallback.

Custom SQL, handwritten repositories, new manual projections, and custom methods
must use the policy-aware APIs themselves. Generated markers do **not** secure
arbitrary SQL or automatically protect other unmarked structs. Apply the resolved
policy to an appropriate builder, or use the existing protected CRUD helpers.
No PostgreSQL RLS migrations, policy-management UI, or frontend authorization is
introduced.

The existing webapp notification rule is preserved: ordinary broadcasts following
protected operations are suppressed unless an access-aware publisher is supplied.
Codegen retains notification options but does not restore unfiltered delivery or
choose recipients. The application-owned publisher must enforce recipient access.

## Tests and release verification

Normal generator tests use the real `gopkg.in/yaml.v3` dependency:

```bash
go test ./...
go vet ./...
```

The generated-application integration fixture is opt-in for ordinary development
and **required before releasing this access integration**:

```bash
CODEGEN_ACCESS_WEBAPP_DIR=/absolute/path/to/policy-webapp \
CODEGEN_ACCESS_MODELBIND_DIR=/absolute/path/to/policy-modelbind \
  go test -count=1 -run TestAccessGeneratedApplicationIntegration -v ./internal/codegen
```

Optional `CODEGEN_ACCESS_DS_DIR` and `CODEGEN_ACCESS_SESSION_DIR` select real local
sources for those transitive modules. The temporary application's `v0.0.0`
requirements are explicitly local-replacement placeholders, not alleged public
policy releases. It runs `go test` and `go vet` on generated models, bindings,
services and HTTP routes with `go 1.25.0`, real libraries, and only a DB I/O capture
seam. It also compiles the handwritten bootstrap example. Unknown or incomplete
local dependency settings fail rather than silently falling back to stubs.

Coverage includes shared marker identity/value pointers/composite keys, explicit
bindings/hidden columns, required enforced inputs, null/zero/absence, path-key
preparation ordering, actual reflected service input types, manual method bodies,
fail-closed paths and notification suppression. Separate generator tests cover
strict YAML, invalid bindings, collisions, regeneration, stale cleanup, enums and
unchanged frontend/legacy behavior.

Standard-library-only syntax/identifier tests can be run even without dependencies:

```bash
cd internal/codegen
go test -count=1 \
  access_identifiers.go access_identifiers_test.go \
  access_signatures.go access_signatures_test.go \
  access_template_syntax_test.go
```

These exercise production identifier/signature helpers and render/parse templates;
they are **not** a substitute for generated application compilation or behavioral
integration with real libraries. See the delivery verification report for checks
actually executed and any environment blockers.
