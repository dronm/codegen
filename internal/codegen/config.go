package codegen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "codegen.yaml"

type Config struct {
	ProjectRoot            string
	ConfigPath             string
	SchemaDir              string
	TemplateDir            string
	ServerRoot             string
	FrontendRoot           string
	MigrationsDir          string
	GoModule               string
	Check                  bool
	GoJSONTagName          string
	GoFilterTagName        string
	BackendEnabled         bool
	FrontendEnabled        bool
	APITestEnabled         bool
	RegistriesEnabled      bool
	MigrationsEnabled      bool
	MigrationsOverwrite    bool
	AllowManualCollisions  bool
	MigrationCreateMode    string
	MigrationCreateBin     string
	MigrationCreateExt     string
	MigrationCreateSeq     bool
	MigrationSequenceWidth int
}

type configFile struct {
	SchemaDir             string                  `yaml:"schemaDir"`
	TemplateDir           string                  `yaml:"templateDir"`
	ServerRoot            string                  `yaml:"serverRoot"`
	FrontendRoot          string                  `yaml:"frontendRoot"`
	MigrationsDir         string                  `yaml:"migrationsDir"`
	GoModule              string                  `yaml:"goModule"`
	GoJSONTagName         string                  `yaml:"goJsonTagName"`
	GoFilterTagName       string                  `yaml:"goFilterTagName"`
	AllowManualCollisions *bool                   `yaml:"allowManualCollisions"`
	Backend               configEnabledSection    `yaml:"backend"`
	Frontend              configEnabledSection    `yaml:"frontend"`
	APITest               configEnabledSection    `yaml:"apiTest"`
	Registries            configEnabledSection    `yaml:"registries"`
	Migrations            configMigrationsSection `yaml:"migrations"`
}

type configEnabledSection struct {
	Enabled *bool `yaml:"enabled"`
}

type configMigrationsSection struct {
	Enabled       *bool  `yaml:"enabled"`
	Overwrite     *bool  `yaml:"overwrite"`
	CreateMode    string `yaml:"createMode"`
	CreateBin     string `yaml:"createBin"`
	CreateExt     string `yaml:"createExt"`
	CreateSeq     *bool  `yaml:"createSeq"`
	SequenceWidth int    `yaml:"sequenceWidth"`
}

func DefaultConfig(projectRoot string) Config {
	return Config{
		ProjectRoot:            cleanPath(projectRoot),
		SchemaDir:              "./schema",
		TemplateDir:            "",
		ServerRoot:             ".",
		FrontendRoot:           "../front",
		MigrationsDir:          "./migrations",
		GoJSONTagName:          "json",
		GoFilterTagName:        "f",
		BackendEnabled:         true,
		FrontendEnabled:        false,
		APITestEnabled:         true,
		RegistriesEnabled:      true,
		MigrationsEnabled:      true,
		MigrationsOverwrite:    false,
		AllowManualCollisions:  false,
		MigrationCreateMode:    "internal",
		MigrationCreateBin:     "migrate",
		MigrationCreateExt:     "sql",
		MigrationCreateSeq:     true,
		MigrationSequenceWidth: 6,
	}
}

// LoadConfig loads codegen.yaml when present, then applies .env, .env.codegen,
// and process-environment overrides. Relative paths are resolved from the
// directory containing codegen.yaml (or the current working directory when no
// config file exists).
func LoadConfig(configPath string) (Config, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = DefaultConfigFile
	}

	absConfigPath, configExists, err := resolveConfigPath(configPath)
	if err != nil {
		return Config{}, err
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}
	if configExists {
		projectRoot = filepath.Dir(absConfigPath)
	}
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve project root: %w", err)
	}

	cfg := DefaultConfig(projectRoot)
	if configExists {
		fileCfg, err := readConfigFile(absConfigPath)
		if err != nil {
			return Config{}, err
		}
		applyConfigFile(&cfg, fileCfg)
		cfg.ConfigPath = absConfigPath
	} else if configPath != DefaultConfigFile && configPath != "./"+DefaultConfigFile {
		return Config{}, fmt.Errorf("config file %s does not exist", configPath)
	}

	values := make(map[string]string)
	for _, filename := range []string{".env", ".env.codegen"} {
		if err := readEnvFile(filepath.Join(projectRoot, filename), values); err != nil {
			return Config{}, err
		}
	}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		values[key] = value
	}

	if err := applyEnvironment(&cfg, values); err != nil {
		return Config{}, err
	}
	if err := finalizeConfig(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func resolveConfigPath(path string) (string, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("resolve config path %s: %w", path, err)
	}
	_, err = os.Stat(absPath)
	if err == nil {
		return absPath, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return absPath, false, nil
	}
	return "", false, fmt.Errorf("stat config file %s: %w", absPath, err)
}

func readConfigFile(path string) (configFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configFile{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg configFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return configFile{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return configFile{}, fmt.Errorf("parse config %s: multiple YAML documents are not supported", path)
	} else if !errors.Is(err, io.EOF) {
		return configFile{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func applyConfigFile(cfg *Config, fileCfg configFile) {
	setString(&cfg.SchemaDir, fileCfg.SchemaDir)
	setString(&cfg.TemplateDir, fileCfg.TemplateDir)
	setString(&cfg.ServerRoot, fileCfg.ServerRoot)
	setString(&cfg.FrontendRoot, fileCfg.FrontendRoot)
	setString(&cfg.MigrationsDir, fileCfg.MigrationsDir)
	setString(&cfg.GoModule, fileCfg.GoModule)
	setString(&cfg.GoJSONTagName, fileCfg.GoJSONTagName)
	setString(&cfg.GoFilterTagName, fileCfg.GoFilterTagName)
	setBool(&cfg.AllowManualCollisions, fileCfg.AllowManualCollisions)
	setBool(&cfg.BackendEnabled, fileCfg.Backend.Enabled)
	setBool(&cfg.FrontendEnabled, fileCfg.Frontend.Enabled)
	setBool(&cfg.APITestEnabled, fileCfg.APITest.Enabled)
	setBool(&cfg.RegistriesEnabled, fileCfg.Registries.Enabled)
	setBool(&cfg.MigrationsEnabled, fileCfg.Migrations.Enabled)
	setBool(&cfg.MigrationsOverwrite, fileCfg.Migrations.Overwrite)
	setString(&cfg.MigrationCreateMode, fileCfg.Migrations.CreateMode)
	setString(&cfg.MigrationCreateBin, fileCfg.Migrations.CreateBin)
	setString(&cfg.MigrationCreateExt, fileCfg.Migrations.CreateExt)
	setBool(&cfg.MigrationCreateSeq, fileCfg.Migrations.CreateSeq)
	if fileCfg.Migrations.SequenceWidth != 0 {
		cfg.MigrationSequenceWidth = fileCfg.Migrations.SequenceWidth
	}
}

func setString(target *string, value string) {
	if strings.TrimSpace(value) != "" {
		*target = strings.TrimSpace(value)
	}
}

func setBool(target *bool, value *bool) {
	if value != nil {
		*target = *value
	}
}

func applyEnvironment(cfg *Config, values map[string]string) error {
	if err := validateBooleanValues(values, []string{
		"CODEGEN_CHECK",
		"CODEGEN_BACKEND_ENABLED",
		"CODEGEN_FRONTEND_ENABLED",
		"CODEGEN_APITEST_ENABLED",
		"CODEGEN_REGISTRIES_ENABLED",
		"CODEGEN_MIGRATIONS_ENABLED",
		"CODEGEN_MIGRATIONS_OVERWRITE",
		"CODEGEN_ALLOW_MANUAL_COLLISIONS",
		"CODEGEN_MIGRATION_CREATE_SEQ",
	}); err != nil {
		return err
	}

	var err error
	cfg.SchemaDir = stringOverride(values, "CODEGEN_SCHEMA_DIR", cfg.SchemaDir)
	cfg.TemplateDir = stringOverrideAllowEmpty(values, "CODEGEN_TEMPLATE_DIR", cfg.TemplateDir)
	cfg.ServerRoot = stringOverride(values, "CODEGEN_SERVER_ROOT", cfg.ServerRoot)
	cfg.FrontendRoot = stringOverride(values, "CODEGEN_FRONTEND_ROOT", cfg.FrontendRoot)
	cfg.MigrationsDir = stringOverride(values, "CODEGEN_MIGRATIONS_DIR", cfg.MigrationsDir)
	cfg.GoModule = stringOverride(values, "CODEGEN_GO_MODULE", cfg.GoModule)
	cfg.GoJSONTagName = stringOverride(values, "CODEGEN_GO_JSON_TAG", cfg.GoJSONTagName)
	cfg.GoFilterTagName = stringOverride(values, "CODEGEN_GO_FILTER_TAG", cfg.GoFilterTagName)
	cfg.MigrationCreateMode = strings.ToLower(stringOverride(values, "CODEGEN_MIGRATION_CREATE_MODE", cfg.MigrationCreateMode))
	cfg.MigrationCreateBin = stringOverride(values, "CODEGEN_MIGRATION_CREATE_BIN", cfg.MigrationCreateBin)
	cfg.MigrationCreateExt = strings.TrimPrefix(stringOverride(values, "CODEGEN_MIGRATION_CREATE_EXT", cfg.MigrationCreateExt), ".")

	cfg.Check = boolOverride(values, "CODEGEN_CHECK", cfg.Check)
	cfg.BackendEnabled = boolOverride(values, "CODEGEN_BACKEND_ENABLED", cfg.BackendEnabled)
	cfg.FrontendEnabled = boolOverride(values, "CODEGEN_FRONTEND_ENABLED", cfg.FrontendEnabled)
	cfg.APITestEnabled = boolOverride(values, "CODEGEN_APITEST_ENABLED", cfg.APITestEnabled)
	cfg.RegistriesEnabled = boolOverride(values, "CODEGEN_REGISTRIES_ENABLED", cfg.RegistriesEnabled)
	cfg.MigrationsEnabled = boolOverride(values, "CODEGEN_MIGRATIONS_ENABLED", cfg.MigrationsEnabled)
	cfg.MigrationsOverwrite = boolOverride(values, "CODEGEN_MIGRATIONS_OVERWRITE", cfg.MigrationsOverwrite)
	cfg.AllowManualCollisions = boolOverride(values, "CODEGEN_ALLOW_MANUAL_COLLISIONS", cfg.AllowManualCollisions)
	cfg.MigrationCreateSeq = boolOverride(values, "CODEGEN_MIGRATION_CREATE_SEQ", cfg.MigrationCreateSeq)
	cfg.MigrationSequenceWidth, err = intOverride(values, "CODEGEN_MIGRATION_SEQUENCE_WIDTH", cfg.MigrationSequenceWidth)
	return err
}

func finalizeConfig(cfg *Config) error {
	cfg.ProjectRoot = cleanPath(cfg.ProjectRoot)
	cfg.SchemaDir = resolveProjectPath(cfg.ProjectRoot, cfg.SchemaDir)
	cfg.ServerRoot = resolveProjectPath(cfg.ProjectRoot, cfg.ServerRoot)
	cfg.FrontendRoot = resolveProjectPath(cfg.ProjectRoot, cfg.FrontendRoot)
	cfg.MigrationsDir = resolveProjectPath(cfg.ProjectRoot, cfg.MigrationsDir)
	if strings.TrimSpace(cfg.TemplateDir) != "" {
		cfg.TemplateDir = resolveProjectPath(cfg.ProjectRoot, cfg.TemplateDir)
	}

	if cfg.GoModule == "" && (cfg.BackendEnabled || cfg.APITestEnabled) {
		modulePath, err := modulePathFromGoMod(filepath.Join(cfg.ServerRoot, "go.mod"))
		if err != nil {
			return err
		}
		cfg.GoModule = modulePath
	}

	if cfg.GoModule == "" && (cfg.BackendEnabled || cfg.APITestEnabled) {
		return fmt.Errorf("CODEGEN_GO_MODULE/goModule is required when backend or API test generation is enabled")
	}
	if cfg.GoJSONTagName == "" {
		return fmt.Errorf("CODEGEN_GO_JSON_TAG/goJsonTagName must not be empty")
	}
	if cfg.GoFilterTagName == "" {
		return fmt.Errorf("CODEGEN_GO_FILTER_TAG/goFilterTagName must not be empty")
	}
	if cfg.MigrationSequenceWidth < 1 || cfg.MigrationSequenceWidth > 12 {
		return fmt.Errorf("migration sequence width must be between 1 and 12")
	}
	if cfg.MigrationsEnabled {
		switch cfg.MigrationCreateMode {
		case "internal":
		case "external":
			if cfg.MigrationCreateBin == "" {
				return fmt.Errorf("migration create binary must not be empty in external mode")
			}
		default:
			return fmt.Errorf("migration create mode must be internal or external")
		}
		if cfg.MigrationCreateExt == "" {
			return fmt.Errorf("migration extension must not be empty when migrations are enabled")
		}
	}
	return nil
}

func resolveProjectPath(projectRoot string, path string) string {
	path = cleanPath(path)
	if filepath.IsAbs(path) {
		return path
	}
	return cleanPath(filepath.Join(projectRoot, path))
}

func modulePathFromGoMod(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("goModule is empty and %s cannot be opened: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		return strings.Trim(fields[1], `"`), nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read module path from %s: %w", path, err)
	}
	return "", fmt.Errorf("goModule is empty and %s has no module directive", path)
}

func readEnvFile(filename string, values map[string]string) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open env file %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=value", filename, lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			return fmt.Errorf("%s:%d: empty key", filename, lineNo)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan env file %s: %w", filename, err)
	}
	return nil
}

func stringOverride(values map[string]string, key string, current string) string {
	value, exists := values[key]
	if !exists || strings.TrimSpace(value) == "" {
		return current
	}
	return strings.TrimSpace(value)
}

func stringOverrideAllowEmpty(values map[string]string, key string, current string) string {
	value, exists := values[key]
	if !exists {
		return current
	}
	return strings.TrimSpace(value)
}

func boolOverride(values map[string]string, key string, current bool) bool {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return current
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return current
	}
	return parsed
}

func validateBooleanValues(values map[string]string, keys []string) error {
	for _, key := range keys {
		value := strings.TrimSpace(values[key])
		if value == "" {
			continue
		}
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be a boolean: %w", key, err)
		}
	}
	return nil
}

func intOverride(values map[string]string, key string, current int) (int, error) {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return current, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func cleanPath(path string) string {
	return filepath.Clean(path)
}
