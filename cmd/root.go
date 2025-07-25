package cmd

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
    Use:   "aiff-tagger",
    Short: "Audio metadata enrichment tool for AIFF files",
    Long: `A CLI tool and daemon for enriching AIFF metadata with record label,
release date, and genre information. Focused on drum & bass but supports
all electronic music genres.`,
    Version: "0.1.0",
}

func Execute() {
    err := rootCmd.Execute()
    if err != nil {
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aiff-tagger/config.yaml)")
    rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")
    rootCmd.PersistentFlags().Bool("dry-run", false, "show what would be done without making changes")

    viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
    viper.BindPFlag("dry-run", rootCmd.PersistentFlags().Lookup("dry-run"))
}

func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)

        configDir := filepath.Join(home, ".aiff-tagger")
        os.MkdirAll(configDir, 0755)

        viper.AddConfigPath(configDir)
        viper.SetConfigType("yaml")
        viper.SetConfigName("config")
    }

    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err == nil {
        if viper.GetBool("verbose") {
            fmt.Println("Using config file:", viper.ConfigFileUsed())
        }
    }

    // Set defaults
    viper.SetDefault("api.musicbrainz.rate_limit", 10)
    viper.SetDefault("api.musicbrainz.user_agent", "aiff-tagger/0.1.0")
    viper.SetDefault("processing.concurrent_workers", 3)
    viper.SetDefault("cache.ttl_hours", 168) // 1 week
}