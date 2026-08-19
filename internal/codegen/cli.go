package codegen

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const usageText = `Usage:
	codegen [generate|check|validate] [options]

Commands:
	generate  Generate files. This is the default command.
	check     Verify that committed generated files match the schemas/templates.
	validate  Validate configuration, schemas, and manual-collision rules only.

Options:
	-config path  Project configuration file (default: codegen.yaml)
	-h            Show help
`

func Run(args []string) error {
	command, rest, err := parseCommand(args)
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("codegen "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", DefaultConfigFile, "project configuration file")
	help := flags.Bool("h", false, "show help")
	flags.BoolVar(help, "help", false, "show help")
	if err := flags.Parse(rest); err != nil {
		return fmt.Errorf("%w\n%s", err, usageText)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s\n%s", strings.Join(flags.Args(), " "), usageText)
	}
	if *help {
		fmt.Fprint(os.Stdout, usageText)
		return nil
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		return err
	}

	switch command {
	case "generate":
		return Generate(cfg)
	case "check":
		cfg.Check = true
		return Generate(cfg)
	case "validate":
		return Validate(cfg)
	default:
		return fmt.Errorf("unknown command %q\n%s", command, usageText)
	}
}

func parseCommand(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "generate", nil, nil
	}
	if strings.HasPrefix(args[0], "-") {
		return "generate", args, nil
	}
	switch args[0] {
	case "generate", "check", "validate":
		return args[0], args[1:], nil
	case "help":
		return "generate", []string{"-h"}, nil
	default:
		return "", nil, fmt.Errorf("unknown command %q\n%s", args[0], usageText)
	}
}
