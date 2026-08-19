# Releasing the generator

The generator should be versioned with normal Go module tags so each consuming project can pin its own version.

## Initial release

After creating the repository at `github.com/dronm/codegen`:

```bash
git init
git add .
git commit -m "Initial standalone code generator"
git tag v0.1.0
git push origin main --tags
```

Then, in each consumer:

```bash
go get -tool github.com/dronm/codegen/cmd/codegen@v0.1.0
go mod tidy
```

## Updating a consumer

Pin an explicit release:

```bash
go get -tool github.com/dronm/codegen/cmd/codegen@v0.2.0
go mod tidy
go tool codegen check
```

If `check` reports drift, run `go tool codegen generate`, review the generated diff, and commit the generator-version update and generated changes together.

## Compatibility policy

Treat changes to any of these as generator API changes:

- accepted YAML properties or validation rules;
- generated Go/TypeScript public names;
- route/permission naming conventions;
- generated file locations;
- template behavior that changes committed output.

Before a breaking schema change, either retain compatibility in the parser or publish it as an intentional major-version migration.
