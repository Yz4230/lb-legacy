/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

type Flags struct {
	RunAsServer bool
	RunAsClient bool
	Port        int
	ConfigPath  string
	LogLevel    string
	Debug       bool
}

var flags Flags

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "legacy",
	Short:         "[従来手法] 流量監視によるSRv6 Segment Listの変更ツール",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(os.Stdout)
		if flags.Debug {
			log.SetLevel(log.DebugLevel)
		} else {
			if logLevel, err := log.ParseLevel(flags.LogLevel); err != nil {
				return errors.Wrap(err, "Failed to parse log level")
			} else {
				log.SetLevel(logLevel)
			}
		}

		if flags.RunAsServer == flags.RunAsClient {
			return errors.New("One of server or client mode must be specified")
		}

		log.Debugf("Command line arguments: %+v", flags)

		if flags.RunAsServer {
			return runServer()
		} else if flags.RunAsClient {
			return runClient()
		}

		return errors.New("Invalid mode")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	// server mode
	rootCmd.Flags().BoolVarP(&flags.RunAsServer, "server", "s", false, "Run as server mode")
	// client mode
	rootCmd.Flags().BoolVarP(&flags.RunAsClient, "client", "c", false, "Run as client mode")
	rootCmd.MarkFlagsMutuallyExclusive("server", "client")

	// running port
	rootCmd.Flags().IntVarP(&flags.Port, "port", "p", 8080, "Port number to run")

	// config file
	rootCmd.Flags().StringVarP(&flags.ConfigPath, "config", "f", "config.yml", "Config file path")

	// Debug mode
	rootCmd.Flags().BoolVarP(&flags.Debug, "debug", "d", false, "Debug mode")
	// log level
	rootCmd.Flags().StringVarP(&flags.LogLevel, "loglevel", "l", "info", "Log level (debug, info, warn, error, fatal)")
}
