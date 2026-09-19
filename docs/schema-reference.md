# Object schema reference

This document describes ordinary CRUD object schemas from `schemaDir`. Accumulation registers use a separate schema contract and directory; see [Accumulation registers](registers.md).

This document defines the YAML object language consumed by `github.com/dronm/codegen` and the output contract of the built-in application profile.

The generator is intentionally aimed at ordinary database-backed CRUD and administrative entities. It generates a coherent unit:

```text
YAML object specification
	↓
PostgreSQL migration draft
	↓
Go model and key model
	↓
Go CRUD service
	↓
HTTP routes and binders
	↓
permission rows and role grants
	↓
optional application-route and main-menu rows
	↓
optional API integration test
	↓
optional Vue/TypeScript administrative scaffold
```

Application-specific workflows remain hand-written. The schema is trusted source code: SQL expressions embedded in schema properties are not sandboxed or interpreted by the generator.

For installation, CLI commands, `codegen.yaml`, environment overrides, and local workspace setup, see [Configuration and CLI](configuration.md) and [Consumer integration](integration.md).

---

## 1. Generated ownership

Generated files use `.gen` in their names and must not be edited manually:

```text
internal/models/<object>.gen.go
internal/services/<object>.gen.go
internal/httpapi/<object>.gen.go
internal/apitest/<object>_test.gen.go
internal/httpapi/routes_gen.go
internal/services/services_gen.go
migrations/<sequence>_<name>.up.sql
migrations/<sequence>_<name>.down.sql
```

Frontend generation is available but disabled by default. It follows the built-in Vue 3/TypeScript/Valibot/PrimeVue application structure. Entries ending in `.gen.ts` are generator-owned; the three `.vue` files are create-once scaffolds and become developer-owned immediately after creation:

```text
<frontend>/src/types/<object>.gen.ts
<frontend>/src/schemas/<object>.gen.ts
<frontend>/src/api/<object>.gen.ts
<frontend>/src/composables/schemas/use<Object>Schemas.gen.ts
<frontend>/src/forms/<object>.gen.ts
<frontend>/src/collections/<object>.gen.ts
<frontend>/src/components/<object>/<Object>Form.vue
<frontend>/src/views/<object>/<Object>List.vue
<frontend>/src/views/<object>/<Object>EditPage.vue
<frontend>/src/router/routeManifest.gen.ts
<frontend>/src/locales/ru.gen.json
```

A separate list projection receives its own modules, matching the base-model/list-projection convention:

```text
<frontend>/src/types/<list-model>.gen.ts
<frontend>/src/schemas/<list-model>.gen.ts
```

The YAML files in the configured `schemaDir` are the source of truth for generated registration. Removing a YAML file removes its route/service call from the generated registry on the next run, but does not delete ordinary stale generated model files. Shared frontend enum artifacts are the exception: their dedicated ownership manifest supports scoped stale-file cleanup; see [Frontend enums](frontend-enums.md).

### Manual extensions

Do not create a hand-written file with the same base name as a generated entity:

```text
internal/services/vehicleBrand.go       # reserved by an existing manual entity
internal/services/vehicleBrand.gen.go   # generated entity
```

Use an explicit extension suffix instead:

```text
internal/services/vehicleBrand_custom.go
internal/httpapi/vehicleBrand_actions.go
internal/models/vehicleBrand_extra.go
```

To replace selected generated CRUD method bodies without taking ownership of the complete service, keep the operations enabled and list their lower-case action names under `service.manualMethods`:

```yaml
crud:
  create: true
  list: true
  detail: true
  update: true
  delete: true

service:
  manualMethods:
    - create
    - update
```

By itself, this setting changes only service-method ownership. The service type, constructor, registration, HTTP routes and binders, permission rows, API types, and frontend contracts remain generated from `crud`. Codegen omits `Create` and `Update` from `internal/services/<object>.gen.go`; implement both methods on the generated service type in `internal/services/<object>_custom.go` using the signatures expected by the generated or application-owned routes.

Supported manual method names are `create`, `list`, `detail`, `update`, and `delete`. Each selected method must also be enabled under `crud`. Before generation, Codegen scans hand-written service files, requires each manually owned receiver method to exist, and rejects receiver methods that collide with methods which are still generator-owned. Generated `.gen.go`, `_gen.go`, and test files do not satisfy manual ownership.

HTTP route ownership is independent from service ownership. Keep each operation in the public CRUD contract, then list the endpoints implemented by the application under `httpRoutes.manualMethods`:

```yaml
crud:
  create: true
  list: true
  detail: true
  update: true
  delete: true

service:
  manualMethods:
    - create
    - detail
    - update

httpRoutes:
  manualMethods:
    - create
    - detail
    - update
```

This aggregate-document example leaves collection `GET` and item `DELETE` routes generator-owned while the application owns `POST`, item `GET`, and `PATCH`. `service.manualMethods` controls Go method bodies; `httpRoutes.manualMethods` controls endpoint and binder generation. Either list can be used without the other.

Set `httpRoutes.enabled: false` when an object should have no generic generated endpoints:

```yaml
httpRoutes:
  enabled: false
```

This is useful for aggregate line models that must only be accessed through their document service. The default is `true`, so omitting `httpRoutes` preserves the previous behavior. Supported `manualMethods` values are `create`, `list`, `detail`, `update`, and `delete`, and each must also be enabled under `crud`. Combining `manualMethods` with `enabled: false` is invalid. Disabling or manually owning generated routes does not change service generation, permission rows, models, API integration tests, or frontend types, schemas, APIs, and forms; those remain driven by `crud`. The generated route registry calls an object's route function only when at least one endpoint remains generator-owned. Application-owned routes must be registered by the application's hand-written route setup. Because an application route may use custom verbs, paths, binders, or helper registrars, `manualMethods` is an ownership declaration; Codegen does not require an exact replacement route to be discoverable in hand-written code.

By default, generation refuses to run when any of these manual files already exists:

```text
internal/models/<object>.go
internal/services/<object>.go
internal/httpapi/<object>.go
```

This prevents duplicate model, service, and route declarations. `CODEGEN_ALLOW_MANUAL_COLLISIONS=true` disables the guard and should only be used during a deliberate ownership migration.

The guard is not limited to matching filenames. Before writing backend output, the generator parses hand-written Go files and rejects collisions with generator-owned declarations and registrations:

- model, key, list-projection, and update-request type names;
- service types, constructors, registration functions, and `MustRegisterService` names;
- HTTP helper/route function names;
- HTTP method/path pairs;
- `WithName` route names;
- `WithPermission` permission names.

An HTTP method/path, `WithName`, or `WithPermission` registration is allowed when its operation is listed under `httpRoutes.manualMethods` or all generated HTTP routes are disabled. Operations that remain generator-owned are still protected from duplicate registration. Use a distinct name for a hand-written route-registration function so that it does not collide with the generated `<object>Routes` function.

Generated `.gen.go` files are excluded from this manual-symbol scan.

---

---

## 2. Current generated backend shape

The built-in templates follow the supported Go/webapp conventions used by Codegen.

### 2.1 Model

A normal identity-key model is generated as:

```go
const productBrandRelation = "public.product_brands"

type ProductBrand struct {
	ID        int       `json:"id" primaryKey:"true" srvCalc:"true"`
	Code      string    `json:"code" required:"true" maxLen:"50"`
	IsActive bool      `json:"is_active" required:"true"`
	CreatedAt time.Time `json:"created_at" srvCalc:"true"`
	UpdatedAt time.Time `json:"updated_at" srvCalc:"true"`
}

func (m ProductBrand) Relation() string {
	return productBrandRelation
}

func (m ProductBrand) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}

type ProductBrandKey struct {
	ID int `json:"id" primaryKey:"true" required:"true"`
}

func (m ProductBrandKey) Relation() string {
	return productBrandRelation
}
```

Supported current tags are:

| YAML property | Go tag |
|---|---|
| `primaryKey: true` | `primaryKey:"true"` |
| `required: true` | `required:"true"` |
| `serverGenerated`, `autoIncrement`, `srvCalc`, or `readOnly` | `srvCalc:"true"` |
| `enum: <name>` | `enum:"<name>"` |
| `maxLen: <n>` | `maxLen:"<n>"` |
| `filter: <expression>` | `f:"<expression>"` |

Legacy generator tags such as `dbRequired` and `dateType` are accepted in old schemas where relevant to SQL/type mapping but are not emitted as modelbind tags.

### 2.2 Service

Generated services use:

```go
type ProductBrandService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
}
```

They register through `webapp.MustRegisterService`, optionally with `webapp.WithCRUDNotifications()`.

Each generated method:

1. checks the session when `sessionRequired` is enabled;
2. checks `ds.Provider` availability;
3. validates path/key values;
4. uses `webapp.InsertModelInput`, `UpdateModelInput`, `DeleteModel`, `FetchModel`, or `FetchCollectionModel`;
5. maps zero affected rows or `ds.ErrNoRows` to `webapp.NotFound`;
6. wraps infrastructure errors with object/action context.

The generated CRUD layer uses `ds/v4` and `webapp` abstractions. It does not introduce direct `pgx` use.

### 2.3 HTTP API

Routes follow the current naming contract:

```text
POST   /objects       <prefix>.create
GET    /objects       <prefix>.list
GET    /objects/{id}  <prefix>.detail
PATCH  /objects/{id}  <prefix>.update
DELETE /objects/{id}  <prefix>.delete
```

All routes for CRUD operations are generated by default. `httpRoutes.manualMethods` removes selected operations from generated HTTP ownership, while `httpRoutes.enabled: false` removes all of them. These settings affect backend route and binder generation only; the CRUD contract and its services, permissions, tests, and frontend artifacts remain available. When no generated endpoints remain, `internal/httpapi/routes_gen.go` omits the object's registration call.

Each route receives:

```go
webapp.WithName("productBrand.list")
webapp.WithPermission("productBrand.list")
webapp.WithService("ProductBrand", "List")
```

Single keys use the standard webapp path/update binders. Composite keys receive generated explicit binders that:

- read every path value;
- validate missing strings;
- parse positive `int` and `int64` values;
- build the full key model;
- combine the key with a validated update body.

This fixes the former generator behavior that treated every entity as a single integer-key resource.

### 2.4 Custom collection projection

An object can select list rows from a view or a different table while create/update use the base table. Detail uses the base table unless `detail` is configured:

```yaml
list:
  model: ProductList
  table:
    schema: public
    name: products_list
  fields:
    - name: id
      type: int
      primaryKey: true
    - name: descr
      type: text
      required: true
```

The generator creates the list model and makes `List` return:

```go
wmodels.CollectionResponse[*models.ProductList]
```

### 2.5 Custom detail projection

Set top-level `detail` using the same shape as `list` (`model`, `table`, `fields`,
and optional `comment`). List and detail projections are independent.

```yaml
detail:
  model: ProductDetail
  table:
    schema: public
    name: products_detail
  fields:
    - name: id
      type: int
      primaryKey: true
    - name: name
      type: text
      required: true
    - name: measure_unit_id
      type: int
      nullable: true
    - name: measure_unit
      type: jsonb
      nullable: true
```

`ProductService.Detail` returns `*models.ProductDetail` and selects from
`public.products_detail` using the existing key parameters, including composite
keys. Create, update, and delete continue to use the base table and model.
Without `detail`, the existing base-model detail behavior is preserved.

The projection model is generated in the object's existing Go model file.
Frontend generation additionally produces `types/productDetail.gen.ts` and
`schemas/productDetail.gen.ts`, including DTO conversion. The API detail result,
collection detail generic, and generated edit page use this projection. Base
create/update types and schemas still derive from top-level `fields`.

Rules:

- `model` must differ from the base model and `list.model`; generated model and
  filename collisions with other objects are rejected.
- `fields` is the complete projection, not a list of extra fields. Include all
  key columns with the same Go types. With frontend generation enabled, key
  JSON names and required TypeScript types must also match the base keys.
- For generated edit forms, include the base writable fields with compatible
  TypeScript types and optionality. Extra reference fields are returned by the
  API; form controls continue to derive from the base fields.
- `nullable`, JSON mappings, type overrides, and Valibot overrides work as for
  list fields. Use `frontend.detailTypeImports` and
  `frontend.detailSchemaImports` for projection-specific imports, with the same
  import-entry shape as `listTypeImports` and `listSchemaImports`.
- Define the SQL view and any reference functions in your project migrations.
  The generator does not create projection views automatically. Each key must
  identify one row in the view.
- If `detail` is in `service.manualMethods`, the custom implementation must
  return the configured projection; the generator still emits its model and
  frontend contract.

See `examples/products_detail.yaml` for a complete object definition. When using
an external template directory, update the Go/Vue templates and include the new
`vue/detailTypes.ts.tmpl` and `vue/detailSchemas.ts.tmpl` files.

---

## 3. Object specification

A complete example is available at:

```text
schema/examples/vehicle_brands.yaml
```

Minimal shape:

```yaml
name: Example
humanName: example
humanNamePlural: examples

table:
  schema: public
  name: examples

route: /examples

keys:
  - name: id
    pathName: id
    type: int

fields:
  - name: id
    type: int
    primaryKey: true
    autoIncrement: true
    serverGenerated: true

  - name: name
    type: string
    required: true
    maxLen: 250

crud:
  create: true
  list: true
  detail: true
  update: true
  delete: true
```

### 3.1 Object properties

| Property | Meaning |
|---|---|
| `name` | Go model name. |
| `comment` | Generated one-line Go model comment. |
| `humanName` | Singular wording used in errors and default permission descriptions. |
| `humanNamePlural` | Plural wording used for collection errors and permissions. |
| `table` | Base PostgreSQL relation, SQL table comment, and table-level checks. |
| `list` | Optional separate collection model/relation. |
| `route` | API collection route. |
| `permissionPrefix` | Permission and route-name prefix; defaults to lower camel model name. |
| `serviceName` | Registered webapp service name; defaults to model name. |
| `service.manualMethods` | Enabled CRUD service methods whose Go implementations are hand-written. |
| `httpRoutes` | Generated backend route ownership. `enabled` defaults to true; `manualMethods` transfers selected enabled CRUD endpoints to application code. |
| `sessionRequired` | Require an authenticated session; defaults to true. |
| `crudNotifications` | Enable webapp CRUD notifications; defaults to true. |
| `keys` | Ordered API/database key fields. |
| `fields` | Base model and table columns. |
| `crud` | Enabled operations. |
| `permissions` | Permission creation and role grants. |
| `applicationRoute` | Optional frontend route registry seed used by a menu item. |
| `menu` | Optional role menu item. |
| `migration` | Migration creation, indexes, and update timestamp trigger. |
| `test` | Optional full-CRUD API integration test. |

### 3.2 Keys

Every key must reference a field marked `primaryKey: true`.

Identity key:

```yaml
keys:
  - name: id
    pathName: id
    type: int
```

Composite natural key:

```yaml
keys:
  - name: group_code
    pathName: groupCode
    type: string
  - name: item_code
    pathName: itemCode
    type: string
```

Supported route key Go types are `string`, `int`, and `int64`.

Natural primary keys are included in create input and excluded from update input. Identity/server-generated keys are excluded from both writable field sets.

### 3.3 Fields

Common field properties:

| Property | Meaning |
|---|---|
| `name` | Lowercase database column name and default JSON field name. Go names preserve project initialisms such as `ID`, `URL`, `API`, and `AI`. |
| `type` | Generator type. |
| `sqlType` | Explicit SQL type override. |
| `goType` | Explicit Go type override. |
| `primaryKey` | Include in key and primary-key SQL. |
| `required` | Required API/model value and `NOT NULL`. |
| `dbRequired` | `NOT NULL` for database-only requirements. |
| `nullable` | Pointer/null TypeScript value. Cannot be combined with `required`. |
| `autoIncrement` | PostgreSQL identity; requires a primary key. |
| `serverGenerated` | Read-only API field with `srvCalc`. |
| `srvCalc` | Explicit server-calculated field. |
| `readOnly` | Exclude from create/update input. |
| `default` | Raw SQL default expression. |
| `json`, `jsonName`, `jsonOmitEmpty` | JSON tag behavior. Writable generic-CRUD fields cannot be JSON-hidden. |
| `filter` | `f` tag value for collection filtering. |
| `enum` | modelbind enum name. |
| `maxLen` | String length tag and default varchar size. |
| `unique` | Inline unique constraint. |
| `check` | Inline SQL check expression. |
| `references` | Foreign key definition. |
| `comment` | One-line Go field comment and PostgreSQL `COMMENT ON COLUMN`. |

Supported built-in types:

```text
int
bigint
float
numeric
string
text
enum
password
bool
json
jsonb
date
time
datetime
timestamptz
```

Use `goType`, `tsType`, `tsDtoType`, `valibot`, and `sqlType` for project-specific types not covered by the built-ins.

Backend generation supports a custom `jsonName`. Generated key models and API tests use the effective JSON name rather than assuming it equals the database column. Frontend scaffolding currently requires JSON-visible fields whose JSON names equal their database names; it fails explicitly otherwise.

Foreign key example:

```yaml
- name: country_id
  type: int
  required: true
  references:
    schema: public
    table: countries
    column: id
    onDelete: RESTRICT
    onUpdate: CASCADE
```

Allowed actions are `NO ACTION`, `RESTRICT`, `CASCADE`, `SET NULL`, and `SET DEFAULT`.

Table, schema, column, reference, and index identifiers are intentionally restricted to lowercase ASCII PostgreSQL identifiers (`[a-z_][a-z0-9_]*`). The templates emit them unquoted, so accepting mixed-case identifiers would otherwise create PostgreSQL case-folding bugs.

---

## 4. Permissions

Permissions default to enabled. One row is generated for every enabled CRUD action:

```text
<prefix>.create
<prefix>.list
<prefix>.detail
<prefix>.update
<prefix>.delete
```

Default role grant:

```yaml
permissions:
  enabled: true
  grantRoles:
    - admin
```

When permission generation is enabled and `grantRoles` is omitted, `admin` is used.

Descriptions can be overridden:

```yaml
permissions:
  descriptions:
    list: Просмотр списка производителей
    detail: Просмотр производителя
    create: Создание производителя
    update: Изменение производителя
    delete: Удаление производителя
```

The migration uses idempotent upserts for `permissions` and `ON CONFLICT DO NOTHING` for `role_permissions`.

Setting `permissions.enabled: false` removes `WithPermission(...)` from generated routes and emits no permission SQL. This should be reserved for genuinely public/system endpoints.

---

## 5. Application routes and menu items

The API route generated under `/api` is not the same thing as an `application_routes` row. `application_routes` describes a frontend page used by the menu constructor.

Example:

```yaml
applicationRoute:
  enabled: true
  name: vehicleBrands
  path: /vehicle-brands
  description: Марки автомобилей
  section: Справочники
  icon: pi pi-car
  menuAvailable: true

menu:
  enabled: true
  role: admin
  caption: Марки автомобилей
  icon: pi pi-car
  sortOrder: 10
  parent:
    caption: Автомобили
    sortOrder: 40
    create: true
```

The migration:

1. upserts the `application_routes` record by name;
2. optionally creates or refreshes a top-level role menu parent;
3. updates an existing role/route menu item to the configured parent, caption, icon, order, and active state;
4. inserts the child menu item when it does not already exist.

Important constraints:

- A generated menu requires `applicationRoute.enabled: true`.
- Generated menu ownership is role-based, not user-specific.
- The parent is matched by role, top-level position, and caption.
- The frontend route synchronization service remains authoritative. Generated page routes are emitted into `src/router/routeManifest.gen.ts`, which the supplied frontend now merges into the compiled route manifest.
- The list route uses the `applicationRoute` name, path, description, section, icon, and `menuAvailable` values. Generated create/edit routes are registered under the `Формы` section and are not menu targets.

---

## 6. Migration generation

Default internal allocation scans the migration directory, finds the largest numeric prefix, and creates the next pair:

```text
000108_create_examples.up.sql
000108_create_examples.down.sql
```

The up migration can generate:

- table and columns;
- inline or composite primary key;
- foreign keys;
- unique and check constraints;
- table-level checks;
- indexes, including expressions and partial predicates;
- an `updated_at` trigger;
- permissions and role grants;
- application route and menu metadata.

The generated migration owns the base table and uses `CREATE TABLE` without `IF NOT EXISTS`. This is intentional: applying a create migration over an existing relation must fail instead of later allowing its down migration to drop a table it did not create.

Example:

```yaml
migration:
  enabled: true
  name: create_vehicle_brands
  updatedAtTrigger: updated_at
  indexes:
    - name: vehicle_brands_name_idx
      columns:
        - lower(name)
    - name: vehicle_brands_active_idx
      columns:
        - is_active
      where: is_active
```

Generated migrations are reviewable drafts. Before applying one, verify:

- column type precision;
- foreign-key delete behavior;
- uniqueness semantics;
- indexes required by collection filters and ordering;
- seed/reference data;
- whether a custom view is needed for list output;
- down-migration data-loss implications;
- route names against the actual frontend manifest.

Existing matching migrations are not overwritten unless `CODEGEN_MIGRATIONS_OVERWRITE=true`.

`CODEGEN_CHECK=true` compares both files in an existing migration pair against the current SQL templates and schema. A missing `.up` or `.down` companion is treated as an incomplete migration and stops generation. Internal pair creation removes any partially created companion if file creation fails.

---

## 7. API integration tests

Test generation is opt-in and requires full CRUD:

```yaml
test:
  enabled: true
  createBody:
    code: TEST
    name: Test value
    is_active: true
  updateBody:
    name: Updated test value
  checkUpdatedField: name
  checkUpdatedValue: Updated test value
```

The generated test performs create, list, detail, update, and delete and constructs an escaped path from the returned key fields.

Do not enable a generated test until its values satisfy all foreign keys and database checks in the integration-test database.

---

## 8. Frontend generation

The built-in frontend templates follow the supported Vue 3/TypeScript/Valibot/PrimeVue application contracts rather than attempting to be a generic Vue scaffold.

### 8.1 Core output

With `CODEGEN_FRONTEND_ENABLED=true`, every active object generates:

```text
src/types/<object>.gen.ts
src/schemas/<object>.gen.ts
src/api/<object>.gen.ts
src/composables/schemas/use<Object>Schemas.gen.ts
```

The API helper uses the real local wrapper:

```ts
createCrudApi({
	basePath,
	serviceName: "ProductBrand",
	getKeyValue,
	fromListDTO,
	fromDetailDTO,
});
```

`serviceName` is mandatory because websocket CRUD notifications and collection refreshes use the backend service name.

The generated Valibot module follows the current `createCommonSchemas(t)` pattern and exports both translated schema factories and default schemas. Date DTO fields are parsed from strings into `Date` model fields.

### 8.2 Separate list projections

A backend list projection is generated as a separate frontend type/schema pair:

```text
src/types/productList.gen.ts
src/schemas/productList.gen.ts
```

The entity API and generated list page import the list model and `fromDTO` function from those modules while retaining the base entity model and key for detail/create/update operations. This matches the existing `Product` and `ProductList` separation.

Use list-specific imports when a view contains reference wrappers, enum values, or other custom types:

```yaml
frontend:
  listTypeImports:
    - from: "@/types/reference"
      names: [Reference]
      typeOnly: true
  listSchemaImports:
    - from: "@/schemas/reference"
      names: [ReferenceSchema]
```

### 8.3 Composite keys

Generated TypeScript types, Valibot key schemas, and CRUD APIs support string, numeric, and mixed composite keys.

For item operations, each key is encoded as its own URL segment:

```ts
const detailPath = (key: VehicleGroupVariantTwinKey): string => {
	return `${basePath}/${encodeURIComponent(String(key.variant_id_a))}/${encodeURIComponent(String(key.variant_id_b))}`;
};
```

The generator overrides `detail`, `update`, and `delete` for composite resources, matching the real `vehicleGroupVariantTwin` API. It does not incorrectly encode the complete `a/b` key as one path value.

Generic `CollectionGrid` page generation remains restricted to a single key because `dataKey` must identify a row unambiguously. Composite-key domain screens remain hand-written, while their types, schemas, and API can still be generated.

### 8.4 Optional forms and pages

Set `frontend.scaffold: true` to create the ordinary built-in CRUD UI. The edit-form scaffold requires `create`, `detail`, and `update`; partial-CRUD entities still receive types, schemas, and APIs, but the unsafe form scaffold is left disabled unless a complete edit lifecycle exists:

```text
src/forms/<object>.gen.ts                 # managed form contract
src/collections/<object>.gen.ts           # managed collection definition
src/components/<object>/<Object>Form.vue   # create-once scaffold, then manual
src/views/<object>/<Object>List.vue        # create-once scaffold, then manual
src/views/<object>/<Object>EditPage.vue    # create-once scaffold, then manual
src/router/routeManifest.gen.ts            # managed
src/locales/ru.gen.json                    # managed
```

`generate` never overwrites an existing scaffold Vue file, and `check` does not compare scaffold contents with the template. This lets field layout, tabs, calculated UI, document actions, and other presentation behavior evolve manually. The `.gen.ts` contracts continue to change with YAML, so TypeScript remains the drift detector for required fields and API/model changes.

The initial form scaffold uses:

- `<script setup lang="ts">`;
- PrimeVue `InputText`, `InputNumber`, `Textarea`, and `Checkbox` controls;
- `FormErrorsView`;
- translated field labels;
- create/edit/copy modes;
- nullable-text normalization;
- a scalar custom-component contract based on `v-model`, `label`, `invalid`, `error`, and `disabled`.

The initial edit-page scaffold uses `useCollectionEditPage`, the generated form contract and translated schemas. Once created, the Vue file is not regenerated. The form emits the complete generated `*New` payload; update delta construction remains library-managed.

The managed collection definition uses `defineCollection`, while the initial list scaffold renders it through `CollectionListPage`. The default `frontend.list.editMode` is `page`. In page mode, create/edit/copy commands are emitted only when form routes exist; a list-only scaffold never points to missing pages. Inline mode keeps create-row/create-model metadata in the managed collection definition.

Default form fields include only JSON-visible writable create fields. Server-calculated IDs and timestamps are not automatically shown. Explicit fields can add read-only values.

Example:

```yaml
frontend:
  scaffold: true
  title: Марки автомобилей

  form:
    columns: 2
    createTitle: Новая марка автомобиля
    editTitle: Марка автомобиля
    copyTitle: Копия марки автомобиля
    fields:
      - field: name
        label: Наименование
        component: text
        autofocus: true

      - field: name_variants
        label: Варианты наименования
        component: textarea
        columnSpan: 2

      - field: popularity_type_id
        label: Популярность
        component: PopularityTypeReferenceInput
        componentImport: "@/components/references/PopularityTypeReferenceInput.vue"

      - field: is_active
        label: Активна
        component: checkbox
        default: true

  list:
    pageSize: 30
    columns:
      - field: id
        label: ID
        width: 8rem
      - field: name
        label: Наименование
        width: 32rem
      - field: is_active
        label: Активна
        width: 8rem

  routes:
    listName: vehicleBrands
    createName: vehicleBrandCreate
    editName: vehicleBrandEdit
```

Custom components must be imported explicitly and must support the same `modelValue` contract as the built-in reference inputs. JSON fields require an explicit JSON editor. Writable date fields require an explicit date component. The generator refuses to emit a misleading plain text control for either case.

### 8.5 Inline collection editing

Set `frontend.list.editMode: inline` to generate a `CollectionGrid` that edits rows directly in the grid. Inline mode disables the generated page edit form by default; set `frontend.form.enabled: true` explicitly if both editing surfaces are required.

Column `label` values are emitted into the generated locale and are used by the collection header. Disabled form scaffolds do not contribute fallback field labels, so an inline list label such as `label: Материал` remains authoritative for that column.


```yaml
frontend:
  scaffold: true

  form:
    enabled: false

  list:
    editMode: inline
    pageSize: 30
    columns:
      - field: id
        label: ID

      - field: inn
        label: INN

      - field: name
        label: Name

      - field: active
        label: Active
```

For inline mode Codegen generates:

- `editMode="inline"` through the typed `GridEditMode` value;
- `editable: true` on writable scalar columns;
- the standard `create` and `edit` grid commands independently of page-form routes;
- a typed `createRow()` draft factory when CRUD create is enabled;
- a typed `createModel()` mapper so the inline draft is converted to the generated `<Object>New` model;
- `createRow` and `createModel` bindings on `CollectionGrid`.

Supported generated inline editors are currently:

- string/text/password/time → text editor;
- enum with a shared definition → enum selector through `dataType: "enum"` and localized `enumOptions` (requires `vue-collection-lib` 0.1.10+);
- int/bigint/float/numeric → numeric editor;
- bool → checkbox editor;
- date → date-only editor through `dataType: "date"`;
- datetime/timestamptz → date-and-time editor through `dataType: "datetime"`.

For date/time fields, the schema type drives the generated grid metadata. A field declared as:

```yaml
- name: created_at
  type: timestamptz
  required: true
  default: "now()"
```

generates an inline column with `dataType: "datetime"`. A plain `type: date` generates `dataType: "date"`. The collection library can therefore use a date-only picker for `date` and a date+time picker for `datetime`/`timestamptz`. You can also set `dataType: datetime` explicitly on a list column when necessary.

Inline draft defaults are derived from safe schema defaults. Boolean and numeric literals are preserved, nullable fields without a default initialize to `null`, and date/datetime/timestamptz fields using `now()` or a `CURRENT_*` default initialize to `new Date()`. Arbitrary SQL expressions are intentionally not translated into TypeScript.

Writable scalar fields are editable by default in inline mode. Override an individual column when needed:

```yaml
frontend:
  list:
    editMode: inline
    columns:
      - field: code
        editable: false
```

`json` and `jsonb` fields are not made editable automatically because the reusable grid requires a domain-specific editor for those values. They may still participate in inline creation when they are nullable or have a schema default. For example, a nullable `ref_1c jsonb` field is initialized to `null` and included in the typed create model without exposing an incorrect text editor.

Reference-backed inline columns can declare the reusable reference configuration directly in YAML:

```yaml
frontend:
  list:
    editMode: inline
    columns:
      - field: material_id
        label: Материал
        editable: true
        reference:
          name: materialReference
          import: "@/references/inventoryReferences"
          field: material
        sortField: "material->>'descr'"
```

This generates the named import and column metadata equivalent to:

```ts
import { materialReference } from "@/references/inventoryReferences";

{
	field: "material_id",
	headerKey: "MaterialStatus.fields.material_id",
	sortField: "material->>'descr'",
	editable: true,
	dataType: "reference",
	reference: materialReference,
	referenceField: "material",
}
```

`reference.name` is the imported TypeScript symbol, `reference.import` is its module path, and `reference.field` is the list-model field that carries the descriptive reference value. Declaring `reference` implies `dataType: "reference"`; an explicitly conflicting `dataType` is rejected. `sortField` is passed through verbatim to the generated grid column so projected/reference descriptions can use the server-side sort expression expected by the collection endpoint. Multiple reference symbols from the same module are grouped into one generated import.

If a writable create field has neither an inline editor nor a safe default (`nullable: true` or `default:`), validation fails rather than generating an inline create flow that cannot produce a valid create model.

Inline editing requires `crud.update: true`. Inline creation additionally requires `crud.create: true`. The generic inline grid remains limited to single-key entities.

See [`examples/customers_inline.yaml`](../examples/customers_inline.yaml) for a complete example.

### 8.6 Custom TypeScript and Valibot types

Non-enum fields can override their transport/model representation:

```yaml
fields:
  - name: supplier
    type: jsonb
    tsType: SupplierReference
    tsDtoType: SupplierReference
    valibotDto: SupplierReferenceSchema
    valibotModel: SupplierReferenceSchema

frontend:
  typeImports:
    - from: "@/types/reference"
      names: [SupplierReference]
      typeOnly: true
  schemaImports:
    - from: "@/schemas/reference"
      names: [SupplierReferenceSchema]
```

`valibotDto` and `valibotModel` are complete expressions. Include `v.nullable`,
`v.array`, or other wrappers when a custom expression needs them.

Declared enum fields now use the shared top-level `enums` definitions rather
than manually importing a string alias. Codegen automatically infers the enum
TypeScript type, adds the required imports, and initializes a locale-aware
picklist schema. Explicit type metadata must agree; explicit validation may wrap
the generated enum schema but must not replace it with generic string validation.

The field property `frontendDefault` supplies a validated enum string literal
for inline creation and is inherited by matching enum list/detail projections.
The column properties `format`, `searchable`, `searchField`, `searchDataType`,
`searchOperations`, and JSON-compatible `editorProps` preserve explicit overrides.
See [Frontend enums](frontend-enums.md) for the full YAML syntax, validation,
nullability/default rules, generated files, dependency check, and database
ownership boundary.

### 8.7 Route and locale integration

The supplied frontend now contains permanent integration points:

```ts
import { generatedRouteManifest } from "@/router/routeManifest.gen";

export const routeManifest = [
	...generatedRouteManifest,
	// hand-written routes
];
```

`src/i18n/index.ts` deep-merges `src/locales/ru.gen.json` after the hand-written Russian messages. Empty generated route and locale files are committed, so the frontend compiles even when no active schema generates pages.

The generator rejects collisions with hand-written frontend files, route names, route paths, and top-level locale keys unless `CODEGEN_ALLOW_MANUAL_COLLISIONS=true` is deliberately used during an ownership migration.

### 8.8 Boundaries

Generated pages are intended for ordinary administrative CRUD. Keep these hand-written:

- tabs and nested editors;
- domain actions and non-CRUD endpoints;
- multi-column or computed row identity;
- complex reference selection requiring a different component contract;
- application-specific multi-entity workflows;
- domain monitoring dashboards;
- bulk operations, imports, exports, and custom dialogs.

Types, schemas, and CRUD APIs can still be generated when page scaffolding is disabled.

---

## 9. Recommended workflow for a new CRUD entity

1. Confirm that the entity is ordinary CRUD. Keep transactional or parser logic hand-written.
2. Verify that no manual model/service/API with the same base name already exists.
3. Copy `schema/examples/vehicle_brands.yaml` into `schema/<entity>.yaml`.
4. Define database keys, fields, constraints, and references first.
5. Select only the CRUD operations that are valid for the entity.
6. Define permissions and explicit descriptions where Russian UI/admin wording matters.
7. Add an application route and menu for a generated list page, or disable `frontend.scaffold` when the UI is domain-specific.
8. Configure frontend field imports, form controls, list columns, and Russian labels when frontend generation is enabled.
9. Run `go tool codegen generate`.
10. Review the SQL migration and generated frontend output manually.
11. Run formatting, Go tests, frontend type checking, and `go tool codegen check` in CI.
12. Add custom behavior in `_custom.go`, `_actions.go`, or another clearly separate file.

---

## 10. What must remain hand-written

Do not force these concerns into the generic templates:

- source downloads, authentication, worker scheduling, and source hashing;
- PostgreSQL LISTEN/NOTIFY consumers;
- domain parsers and rule-engine execution;
- cross-aggregate create/update reconciliation;
- manual override precedence;
- multi-table transactions and row locking;
- image storage and transformations;
- cache invalidation with injected dependencies;
- custom authorization logic beyond a normal permission code;
- bulk actions, imports, exports, and asynchronous jobs;
- complex list views whose SQL must be designed explicitly.

A generated CRUD service can coexist with separate custom services, but the registration names and route names must remain unique.

---

## 11. Known boundaries

- Active schema files are processed only from the top level of `CODEGEN_SCHEMA_DIR`; subdirectories are ignored.
- Stale generated object files are not deleted automatically.
- Composite-key types/schemas/APIs and separate list projections are supported; generic `CollectionGrid` and edit-page generation still require one key.
- Migration generation does not infer seed data, views, triggers other than `updated_at`, or domain procedures.
- One object specification generates one base table and at most one separate list projection.
- Application-route migration data and generated frontend manifest entries must still be synchronized by the normal frontend/backend route synchronization flow.
- Generated errors use `humanName`; choose it carefully.
- Custom model methods or fields belong in separate files and must not duplicate generated declarations.

---

## 12. Schema validation and failure policy

Generation stops before writing object files when a schema is structurally unsafe. Validation includes:

- every primary-key field must appear exactly once in `keys`;
- every YAML property must be recognized and each file must contain exactly one document;
- key Go types must match their corresponding model field types;
- duplicate database names, generated Go names, JSON names, and path parameter names are rejected;
- table, column, reference, index, and migration identifiers must be safe unquoted SQL identifiers;
- collection API routes cannot contain placeholders, whitespace, or control characters;
- application route names must be identifiers and application paths must be valid absolute paths;
- primary keys cannot be nullable or JSON-hidden;
- writable create/update fields cannot use `json: false` because generic binders could not populate them;
- update generation requires at least one writable non-key field;
- index methods and foreign-key actions are restricted to supported PostgreSQL values;
- `test.enabled` requires full CRUD;
- a menu requires an enabled application route;
- duplicate generated filenames, model names, service names, routes, permission prefixes, relations, application route names, and migration names are rejected across active schemas;
- generated backend ownership cannot overlap existing hand-written declarations, service registrations, HTTP method/path pairs, route names, permission names, or reserved base files unless `CODEGEN_ALLOW_MANUAL_COLLISIONS=true` is deliberately set;
- generated frontend ownership cannot overlap hand-written type/schema/API/composable/page files, route names, route paths, or locale keys;
- frontend JSON property names must be valid TypeScript identifiers;
- custom frontend components require explicit imports, and unsupported JSON/writable-date default controls are rejected.

Raw SQL expressions in defaults, checks, expression indexes, and partial-index predicates are intentionally not interpreted. YAML schemas are trusted source code and these expressions still require manual review.
