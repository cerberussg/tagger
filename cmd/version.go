package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print version information",
    Long:  "Print the version number of tagger",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("tagger v0.1.0")
        fmt.Println("Audio metadata enrichment tool")
        fmt.Println("Built for finding Record Label, Release Date, and Genre in AIFF files")
    },
}

func init() {
    rootCmd.AddCommand(versionCmd)
}