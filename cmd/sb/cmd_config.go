package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"servicebus/internal/config"
)

func buildConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage contexts and settings",
		RunE:  runConfigShow,
	}
	cmd.AddCommand(
		buildSetContextCmd(),
		buildUseContextCmd(),
		buildListContextsCmd(),
		buildDeleteContextCmd(),
		buildOpenConfigCmd(),
	)
	return cmd
}

func runConfigShow(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	fmt.Printf("Config file: %s\n\n", config.ConfigPath())
	fmt.Printf("Current context: %s\n\n", cfg.CurrentContext)
	fmt.Println("Contexts:")
	for name, ctx := range cfg.Contexts {
		marker := "  "
		if name == cfg.CurrentContext {
			marker = "* "
		}
		if ctx.ConnectionString != "" {
			fmt.Printf("%s%s  (connection string)\n", marker, name)
		} else {
			fmt.Printf("%s%s  (namespace: %s)\n", marker, name, ctx.Namespace)
		}
	}
	fmt.Printf("\nSettings:\n")
	fmt.Printf("  max-items:             %d\n", cfg.Settings.MaxItems)
	fmt.Printf("  working-directory:     %s\n", cfg.Settings.WorkingDirectory)
	fmt.Printf("  cache-expiration-days: %d\n", cfg.Settings.CacheExpirationDays)
	fmt.Printf("  thresholds.size-%%:    %d\n", cfg.Settings.Thresholds.SizePercentage)
	fmt.Printf("  thresholds.last-access-days: %d\n", cfg.Settings.Thresholds.LastAccessDays)
	return nil
}

func buildSetContextCmd() *cobra.Command {
	var connStr, ns string
	var useDefault bool
	cmd := &cobra.Command{
		Use:   "set-context <name>",
		Short: "Add or update a named context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.Contexts[args[0]] = config.Context{
				ConnectionString:      connStr,
				Namespace:             ns,
				UseDefaultCredentials: useDefault,
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Context %q saved.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&connStr, "connection-string", "", "Connection string")
	cmd.Flags().StringVar(&ns, "namespace", "", "Azure AD namespace")
	cmd.Flags().BoolVar(&useDefault, "use-default-credentials", false, "Use DefaultAzureCredential")
	return cmd
}

func buildUseContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-context <name>",
		Short: "Set the default context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Contexts[args[0]]; !ok {
				return fmt.Errorf("context %q not found", args[0])
			}
			cfg.CurrentContext = args[0]
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Switched to context %q.\n", args[0])
			return nil
		},
	}
}

func buildListContextsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all contexts",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			for name := range cfg.Contexts {
				marker := "  "
				if name == cfg.CurrentContext {
					marker = "* "
				}
				fmt.Printf("%s%s\n", marker, name)
			}
			return nil
		},
	}
}

func buildDeleteContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-context <name>",
		Short: "Remove a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Contexts[args[0]]; !ok {
				return fmt.Errorf("context %q not found", args[0])
			}
			delete(cfg.Contexts, args[0])
			if cfg.CurrentContext == args[0] {
				cfg.CurrentContext = ""
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Context %q deleted.\n", args[0])
			return nil
		},
	}
}

func buildOpenConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open the config file (contains saved connection strings)",
		RunE: func(_ *cobra.Command, _ []string) error {
			path := config.ConfigPath()

			// Ensure the config file exists before opening it.
			if _, err := os.Stat(path); os.IsNotExist(err) {
				cfg, loadErr := config.Load()
				if loadErr != nil {
					return loadErr
				}
				if saveErr := config.Save(cfg); saveErr != nil {
					return saveErr
				}
			} else if err != nil {
				return err
			}

			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				cmd = exec.Command("open", path)
			case "windows":
				cmd = exec.Command("cmd", "/c", "start", "", path)
			default:
				cmd = exec.Command("xdg-open", path)
			}

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("open config file %q: %w", path, err)
			}

			fmt.Printf("Opened config file: %s\n", path)
			return nil
		},
	}
}
