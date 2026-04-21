package cmd

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/zixbaka/ishtrak/internal/config"
)

var (
	cfgPath string
	cfg     *config.Config
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "ishtrak",
	Short: "Ishtrak — task management CLI for Claude Code",
	Long: `Ishtrak lets Claude Code manage tasks on any project management platform
via the browser extension — no MCP required.

Quick start:
  ishtrak init              Set up ishtrak for the first time
  ishtrak task list         List tasks in your project
  ishtrak task create       Create a new task
  ishtrak task get PROJ-1   Get task details
  ishtrak task update PROJ-1 --status "In Progress"`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		level := zerolog.WarnLevel
		if verbose {
			level = zerolog.DebugLevel
		}
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
			Level(level).
			With().Timestamp().Logger()

		if cmd == initCmd || cmd == daemonCmd {
			return nil
		}
		var err error
		cfg, err = config.Load(cfgPath)
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", config.DefaultConfigPath(), "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(hostCmd)
	rootCmd.AddCommand(daemonCmd)
}
