package cmd

import (
	"fmt"
	"os"

	"github.com/mikumaycry/akari/internal/config"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string
var version string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "akari",
	Short: "An invisible proxy",
	Long:  `An invisible proxy that gains power from Akaza Akari`,
	RunE:  run,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(v string) {
	version = v
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	initFlags()
	initCmds()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	config.C.Version = version

	viper.SetConfigType("json")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Use default config location.
		viper.SetConfigFile("/etc/akari/akari.json")
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Unmashal config
	if err := viper.Unmarshal(&config.C); err != nil {
		fmt.Printf("Unmarshal config: %s Config file: %s\n", err, viper.ConfigFileUsed())
		os.Exit(1)
	}
}

func initFlags() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is /etc/akari/akari.json)")
	// Bind cmdline param log-level with config param log.level
	rootCmd.PersistentFlags().Int("log-level", 4, "debug=5, info=4, warn=3, error=2, fatal=1, panic=0")
	viper.BindPFlag("logLevel", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.SetDefault("mode", "server")
	viper.SetDefault("addr", "0.0.0.0:443")
	viper.SetDefault("httpRedirect", false)
	viper.SetDefault("conf", "/etc/akari/conf")
	// TLS Config
	viper.SetDefault("tls.fs", false)
}

func initCmds() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
}
