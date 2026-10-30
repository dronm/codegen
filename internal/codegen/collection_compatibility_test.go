package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The fixture and hashes were captured by running the unmodified input release
// against its original vehicle_brands.yaml example. This checks the historical
// output itself, rather than comparing two branches of today's templates.
func TestCollectionLegacyOutputMatchesOriginalRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := DefaultConfig(root)
	cfg.SchemaDir = filepath.Join(root, "schema")
	cfg.ServerRoot = filepath.Join(root, "server")
	cfg.FrontendRoot = filepath.Join(root, "front")
	cfg.GoModule = "example.com/legacy"
	cfg.FrontendEnabled = true
	cfg.APITestEnabled = false
	cfg.RegistersEnabled = false
	cfg.MigrationsEnabled = false
	if err := os.MkdirAll(cfg.SchemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("testdata/collection/legacy-vehicle-brands.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.SchemaDir, "vehicle_brands.yaml"), schema, 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/collection/legacy-output.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]string
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	for generation := 0; generation < 2; generation++ {
		if err := Generate(cfg); err != nil {
			t.Fatal(err)
		}
		actual := make(map[string]string)
		for _, directory := range []string{cfg.ServerRoot, cfg.FrontendRoot} {
			if err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				digest := sha256.Sum256(content)
				actual[filepath.ToSlash(name)] = hex.EncodeToString(digest[:])
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(actual, expected) {
			for name, hash := range expected {
				if actual[name] != hash {
					t.Errorf("legacy file %s changed: SHA256 %s, want %s", name, actual[name], hash)
				}
			}
			for name := range actual {
				if _, exists := expected[name]; !exists {
					t.Errorf("unexpected generated file %s", name)
				}
			}
			t.Fatal("schemas without collection options must retain their original generated output")
		}
	}
	cfg.Check = true
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}
}
