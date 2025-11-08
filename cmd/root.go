package cmd

import (
	"fmt"
	"os"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mcpfurl",
	Short: "mcpfurl - Fetch a web page, download an image, or perform a web search",
}

var debugCmd = &cobra.Command{
	Use:    "debug",
	Short:  "Show some debug information",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("-- Configuration --")
		fmt.Printf("config file    : %s\n", configFilePath)
		if userConfig == nil {
			fmt.Println("(no config loaded)")
		} else {
			if userConfig.MCP != nil {
				fmt.Println("[mcp]")
				if userConfig.MCP.WebDriverPort != nil {
					fmt.Printf("  web_driver_port: %d\n", *userConfig.MCP.WebDriverPort)
				}
				if userConfig.MCP.WebDriverPath != nil {
					fmt.Printf("  web_driver_path: %s\n", *userConfig.MCP.WebDriverPath)
				}
				if userConfig.MCP.WebDriverLog != nil {
					fmt.Printf("  web_driver_log : %s\n", *userConfig.MCP.WebDriverLog)
				}
				if userConfig.MCP.UsePandoc != nil {
					fmt.Printf("  use_pandoc     : %t\n", *userConfig.MCP.UsePandoc)
				}
				if userConfig.MCP.SearchEngine != nil {
					fmt.Printf("  search_engine  : %s\n", *userConfig.MCP.SearchEngine)
				}
				if userConfig.MCP.Verbose != nil {
					fmt.Printf("  verbose        : %t\n", *userConfig.MCP.Verbose)
				}
			}
			if userConfig.HTTP != nil {
				fmt.Println("[http]")
				if userConfig.HTTP.Addr != nil {
					fmt.Printf("  addr.     : %s\n", *userConfig.HTTP.Addr)
				}
				if userConfig.HTTP.Port != nil {
					fmt.Printf("  port.     : %d\n", *userConfig.HTTP.Port)
				}
				if userConfig.HTTP.MasterKey != nil {
					fmt.Printf("  master_key: ********\n")
				}
			}
			if userConfig.GoogleCustom != nil {
				fmt.Println("[google_custom]")
				if userConfig.GoogleCustom.Cx != nil {
					fmt.Printf("  cx : %s\n", *userConfig.GoogleCustom.Cx)
				}
				if userConfig.GoogleCustom.Key != nil {
					fmt.Printf("  key: %s\n", *userConfig.GoogleCustom.Key)
				}
			}
			if userConfig.Cache != nil {
				fmt.Println("[cache]")
				if userConfig.Cache.SearchDB != nil {
					fmt.Printf("  db_path : %s\n", *userConfig.Cache.SearchDB)
				}
				if userConfig.Cache.Expires != nil {
					fmt.Printf("  expires: %s\n", *userConfig.Cache.Expires)
				}
			}
		}

		applyMCPConfig(cmd)
		applyMCPHTTPConfig(cmd)
		applyGoogleCustomConfig(cmd)
		applyCacheConfig(cmd)

		fmt.Println("\n-- Effective Flags --")
		fmt.Printf("web_driver_port: %d\n", webDriverPort)
		fmt.Printf("web_driver_path: %s\n", webDriverPath)
		fmt.Printf("web_driver_log : %s\n", webDriverLog)
		fmt.Printf("use_pandoc     : %t\n", usePandoc)
		fmt.Printf("verbose        : %t\n", verbose)
		fmt.Printf("search_engine  : %s\n", searchEngine)
		fmt.Printf("search_cache   : %s\n", searchCachePath)
		fmt.Printf("cache_expires  : %s\n", searchCacheExpiresStr)
		fmt.Printf("google_cx      : %s\n", googleCx)
		fmt.Printf("google_key     : %s\n", googleKey)
		if userConfig.HTTP.MasterKey != nil && *userConfig.HTTP.MasterKey != "" {
			fmt.Printf("master_key     : ********\n")
		} else {
			fmt.Printf("master_key     : \n")
		}
		fmt.Printf("mcp_addr       : %s\n", mcpAddr)
		fmt.Printf("mcp_port       : %d\n", mcpPort)
		fmt.Printf("image_max_bytes: %d\n", fetchurl.DefaultMaxDownloadBytes)
	},
}

var licenseCmd = &cobra.Command{
	Use:    "license",
	Short:  "Show the license",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(licenseText)
	},
}

var licenseText string

func SetLicenseText(txt string) {
	licenseText = txt
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	configUsage := "Path to config file (YAML). You can also set MCPFURL_CONFIG."
	rootCmd.PersistentFlags().StringVar(&configFilePath, "config", "", configUsage)
	cobra.OnInitialize(loadConfigFile)

	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(licenseCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
