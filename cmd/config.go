package cmd

import (
    "fmt"
    "strings"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Manage configuration settings",
    Long: `View and modify configuration settings for aiff-tagger.
    
Settings are stored in ~/.aiff-tagger/config.yaml`,
}

var configSetCmd = &cobra.Command{
    Use:   "set <key> <value>",
    Short: "Set a configuration value",
    Long: `Set a configuration key to a specific value.

Available keys:
  api.musicbrainz.rate_limit    - API calls per minute (default: 10)
  api.musicbrainz.user_agent    - User agent for API requests
  processing.concurrent_workers - Number of parallel workers (default: 3)
  cache.ttl_hours              - Cache TTL in hours (default: 168)
  watch_dirs                   - Comma-separated list of directories to watch

Examples:
  aiff-tagger config set api.musicbrainz.rate_limit 15
  aiff-tagger config set watch_dirs "~/Music/DnB,~/Downloads"`,
    Args: cobra.ExactArgs(2),
    Run:  runConfigSet,
}

var configShowCmd = &cobra.Command{
    Use:   "show [key]",
    Short: "Show configuration values",
    Long: `Display current configuration settings. If no key is specified,
shows all settings.

Examples:
  aiff-tagger config show
  aiff-tagger config show api.musicbrainz.rate_limit`,
    Args: cobra.MaximumNArgs(1),
    Run:  runConfigShow,
}

func init() {
    rootCmd.AddCommand(configCmd)
    configCmd.AddCommand(configSetCmd)
    configCmd.AddCommand(configShowCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) {
    key := args[0]
    value := args[1]
    
    // Handle comma-separated lists
    if strings.Contains(value, ",") {
        values := strings.Split(value, ",")
        for i, v := range values {
            values[i] = strings.TrimSpace(v)
        }
        viper.Set(key, values)
    } else {
        viper.Set(key, value)
    }
    
    err := viper.WriteConfig()
    if err != nil {
        // Try to write to default location if config doesn't exist
        err = viper.SafeWriteConfig()
        if err != nil {
            fmt.Printf("Error writing config: %v\n", err)
            return
        }
    }
    
    fmt.Printf("Set %s = %v\n", key, viper.Get(key))
}

func runConfigShow(cmd *cobra.Command, args []string) {
    if len(args) == 1 {
        // Show specific key
        key := args[0]
        value := viper.Get(key)
        if value == nil {
            fmt.Printf("Key '%s' is not set\n", key)
            return
        }
        fmt.Printf("%s = %v\n", key, value)
    } else {
        // Show all settings
        fmt.Println("Current configuration:")
        fmt.Printf("Config file: %s\n\n", viper.ConfigFileUsed())
        
        settings := map[string]interface{}{
            "api.musicbrainz.rate_limit":    viper.Get("api.musicbrainz.rate_limit"),
            "api.musicbrainz.user_agent":    viper.Get("api.musicbrainz.user_agent"),
            "processing.concurrent_workers": viper.Get("processing.concurrent_workers"),
            "cache.ttl_hours":              viper.Get("cache.ttl_hours"),
            "watch_dirs":                   viper.Get("watch_dirs"),
        }
        
        for key, value := range settings {
            fmt.Printf("%-30s = %v\n", key, value)
        }
    }
}