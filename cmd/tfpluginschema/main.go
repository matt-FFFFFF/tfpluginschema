package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	cli "github.com/urfave/cli/v3"

	"github.com/matt-FFFFFF/tfpluginschema"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	cmd := buildRootCommand()
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// buildRootCommand constructs the full CLI command tree.
func buildRootCommand() *cli.Command {
	return &cli.Command{
		Name:    "tfpluginschema",
		Usage:   "Query Terraform/OpenTofu provider schemas from the registry",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "namespace",
				Aliases:  []string{"n"},
				Usage:    "Provider namespace (e.g. hashicorp, Azure)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"p"},
				Usage:    "Provider name (e.g. aws, azapi)",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "provider-version",
				Aliases: []string{"pv"},
				Usage:   "Provider version or constraint (e.g. 2.5.0, ~>2.1). Empty for latest",
			},
			&cli.StringFlag{
				Name:    "registry",
				Aliases: []string{"r"},
				Usage:   "Registry type: opentofu (default) or terraform",
				Value:   "opentofu",
			},
			&cli.StringFlag{
				Name:    "cache-dir",
				Usage:   "Directory used to cache downloaded providers (overrides $" + tfpluginschema.EnvCacheDir + ")",
				Sources: cli.EnvVars(tfpluginschema.EnvCacheDir),
			},
			&cli.BoolFlag{
				Name:  "force-fetch",
				Usage: "Always download the provider, bypassing the local cache",
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Suppress cache hit/miss status messages on stderr",
			},
		},
		Commands: []*cli.Command{
			providerCommand(),
			resourceCommand(),
			datasourceCommand(),
			functionCommand(),
			ephemeralCommand(),
			versionCommand(),
		},
	}
}

// requestFromCmd builds a tfpluginschema.Request from the CLI flags.
func requestFromCmd(cmd *cli.Command) tfpluginschema.Request {
	return tfpluginschema.Request{
		Namespace:    cmd.String("namespace"),
		Name:         cmd.String("name"),
		Version:      cmd.String("provider-version"),
		RegistryType: registryTypeFromString(cmd.String("registry")),
	}
}

// versionsRequestFromCmd builds a tfpluginschema.VersionsRequest from the CLI flags.
func versionsRequestFromCmd(cmd *cli.Command) tfpluginschema.VersionsRequest {
	return tfpluginschema.VersionsRequest{
		Namespace:    cmd.String("namespace"),
		Name:         cmd.String("name"),
		RegistryType: registryTypeFromString(cmd.String("registry")),
	}
}

// registryTypeFromString converts a string to a RegistryType.
func registryTypeFromString(s string) tfpluginschema.RegistryType {
	switch strings.ToLower(s) {
	case "terraform":
		return tfpluginschema.RegistryTypeTerraform
	default:
		return tfpluginschema.RegistryTypeOpenTofu
	}
}

// newServer creates a new tfpluginschema.Server with minimal logging and
// configures it from the CLI flags (cache dir, force fetch, status reporting).
func newServer(cmd *cli.Command) *tfpluginschema.Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	opts := []tfpluginschema.ServerOption{
		tfpluginschema.WithCacheDir(cmd.String("cache-dir")),
		tfpluginschema.WithForceFetch(cmd.Bool("force-fetch")),
	}
	if !cmd.Bool("quiet") {
		opts = append(opts, tfpluginschema.WithCacheStatusFunc(func(req tfpluginschema.Request, status tfpluginschema.CacheStatus) {
			switch status {
			case tfpluginschema.CacheStatusHit:
				fmt.Fprintf(os.Stderr, "cache hit: %s/%s %s\n", req.Namespace, req.Name, req.Version)
			case tfpluginschema.CacheStatusMiss:
				fmt.Fprintf(os.Stderr, "downloading: %s/%s %s\n", req.Namespace, req.Name, req.Version)
			}
		}))
	}
	return tfpluginschema.NewServer(logger, opts...)
}

// printJSON marshals v as indented JSON and writes it to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printList writes each string in items to stdout, one per line.
func printList(items []string) {
	for _, item := range items {
		fmt.Println(item)
	}
}

// --- provider ---

func providerCommand() *cli.Command {
	return &cli.Command{
		Name:  "provider",
		Usage: "Query the provider configuration schema",
		Commands: []*cli.Command{
			{
				Name:  "schema",
				Usage: "Get the provider configuration schema",
				Action: func(_ context.Context, cmd *cli.Command) error {
					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					schema, err := s.GetProviderSchema(req)
					if err != nil {
						return err
					}
					return printJSON(schema)
				},
			},
		},
	}
}

// --- resource ---

func resourceCommand() *cli.Command {
	return &cli.Command{
		Name:  "resource",
		Usage: "Query resource schemas",
		Commands: []*cli.Command{
			{
				Name:      "schema",
				Usage:     "Get the schema for a specific resource",
				ArgsUsage: "<resource-name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					args := cmd.Args()
					if args.Len() < 1 {
						return fmt.Errorf("resource name is required as an argument")
					}
					resourceName := args.First()

					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					schema, err := s.GetResourceSchema(req, resourceName)
					if err != nil {
						return err
					}
					return printJSON(schema)
				},
			},
			{
				Name:  "list",
				Usage: "List all resource names",
				Action: func(_ context.Context, cmd *cli.Command) error {
					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					resources, err := s.ListResources(req)
					if err != nil {
						return err
					}
					printList(resources)
					return nil
				},
			},
		},
	}
}

// --- datasource ---

func datasourceCommand() *cli.Command {
	return &cli.Command{
		Name:  "datasource",
		Usage: "Query data source schemas",
		Commands: []*cli.Command{
			{
				Name:      "schema",
				Usage:     "Get the schema for a specific data source",
				ArgsUsage: "<datasource-name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					args := cmd.Args()
					if args.Len() < 1 {
						return fmt.Errorf("data source name is required as an argument")
					}
					dsName := args.First()

					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					schema, err := s.GetDataSourceSchema(req, dsName)
					if err != nil {
						return err
					}
					return printJSON(schema)
				},
			},
			{
				Name:  "list",
				Usage: "List all data source names",
				Action: func(_ context.Context, cmd *cli.Command) error {
					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					dataSources, err := s.ListDataSources(req)
					if err != nil {
						return err
					}
					printList(dataSources)
					return nil
				},
			},
		},
	}
}

// --- function ---

func functionCommand() *cli.Command {
	return &cli.Command{
		Name:  "function",
		Usage: "Query provider function schemas",
		Commands: []*cli.Command{
			{
				Name:      "schema",
				Usage:     "Get the schema for a specific function",
				ArgsUsage: "<function-name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					args := cmd.Args()
					if args.Len() < 1 {
						return fmt.Errorf("function name is required as an argument")
					}
					funcName := args.First()

					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					schema, err := s.GetFunctionSchema(req, funcName)
					if err != nil {
						return err
					}
					return printJSON(schema)
				},
			},
			{
				Name:  "list",
				Usage: "List all function names",
				Action: func(_ context.Context, cmd *cli.Command) error {
					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					functions, err := s.ListFunctions(req)
					if err != nil {
						return err
					}
					printList(functions)
					return nil
				},
			},
		},
	}
}

// --- ephemeral ---

func ephemeralCommand() *cli.Command {
	return &cli.Command{
		Name:  "ephemeral",
		Usage: "Query ephemeral resource schemas",
		Commands: []*cli.Command{
			{
				Name:      "schema",
				Usage:     "Get the schema for a specific ephemeral resource",
				ArgsUsage: "<ephemeral-resource-name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					args := cmd.Args()
					if args.Len() < 1 {
						return fmt.Errorf("ephemeral resource name is required as an argument")
					}
					ephName := args.First()

					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					schema, err := s.GetEphemeralResourceSchema(req, ephName)
					if err != nil {
						return err
					}
					return printJSON(schema)
				},
			},
			{
				Name:  "list",
				Usage: "List all ephemeral resource names",
				Action: func(_ context.Context, cmd *cli.Command) error {
					s := newServer(cmd)
					defer s.Cleanup()

					req := requestFromCmd(cmd)
					ephemeralResources, err := s.ListEphemeralResources(req)
					if err != nil {
						return err
					}
					printList(ephemeralResources)
					return nil
				},
			},
		},
	}
}

// --- version ---

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Query available provider versions",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List available versions for the provider",
				Action: func(_ context.Context, cmd *cli.Command) error {
					s := newServer(cmd)
					defer s.Cleanup()

					req := versionsRequestFromCmd(cmd)
					versions, err := s.GetAvailableVersions(req)
					if err != nil {
						return err
					}
					for _, v := range versions {
						fmt.Println(v.Original())
					}
					return nil
				},
			},
		},
	}
}
