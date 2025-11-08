package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/spf13/cobra"
)

var (
	fetchImgOutput   string
	fetchImgMaxBytes int
)

var fetchImgCmd = &cobra.Command{
	Use:   "fetch-img <url>",
	Short: "Download an image or binary file and save it to disk",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]

		var logger *slog.Logger
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}
		var fetcher *fetchurl.WebFetcher
		var err error
		fetcher, err = fetchurl.NewWebFetcher(fetchurl.WebFetcherOptions{
			Logger:           logger,
			MaxDownloadBytes: fetchImgMaxBytes,
		})
		if err != nil {
			log.Fatalf("error setting up webfetcher %s: %v", url, err)
		}

		ctx := context.Background()
		resource, err := fetcher.DownloadResource(ctx, url)
		if err != nil {
			log.Fatalf("error downloading %s: %v", url, err)
		}

		if fetchImgOutput == "-" {
			if _, err := os.Stdout.Write(resource.Body); err != nil {
				log.Fatalf("error writing to stdout: %v", err)
			}
			return
		}

		if err := os.WriteFile(fetchImgOutput, resource.Body, 0o644); err != nil {
			log.Fatalf("error writing %s: %v", fetchImgOutput, err)
		}

		fmt.Printf("Saved %d bytes to %s (content-type: %s)\n", len(resource.Body), fetchImgOutput, resource.ContentType)
	},
}

func init() {
	fetchImgCmd.Flags().StringVarP(&fetchImgOutput, "output", "o", "", "Path to save the downloaded file or '-' for stdout")
	fetchImgCmd.Flags().IntVar(&fetchImgMaxBytes, "max-bytes", fetchurl.DefaultMaxDownloadBytes, "Maximum download size in bytes (0 means unlimited)")
	fetchImgCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	fetchImgCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(fetchImgCmd)
}
