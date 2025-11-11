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
			if userConfig.MCPFurlCfg != nil {
				fmt.Println("[mcp]")
				if userConfig.MCPFurlCfg.WebDriverPort != nil {
					fmt.Printf("  web_driver_port: %d\n", *userConfig.MCPFurlCfg.WebDriverPort)
				}
				if userConfig.MCPFurlCfg.WebDriverPath != nil {
					fmt.Printf("  web_driver_path: %s\n", *userConfig.MCPFurlCfg.WebDriverPath)
				}
				// if userConfig.MCP.WebDriverLog != nil {
				// 	fmt.Printf("  web_driver_log : %s\n", *userConfig.MCP.WebDriverLog)
				// }
				if userConfig.MCPFurlCfg.UsePandoc != nil {
					fmt.Printf("  use_pandoc     : %t\n", *userConfig.MCPFurlCfg.UsePandoc)
				}
				if userConfig.MCPFurlCfg.SearchEngine != nil {
					fmt.Printf("  search_engine  : %s\n", *userConfig.MCPFurlCfg.SearchEngine)
				}
				if userConfig.MCPFurlCfg.Verbose != nil {
					fmt.Printf("  verbose        : %t\n", *userConfig.MCPFurlCfg.Verbose)
				}
				if len(userConfig.MCPFurlCfg.Allow) > 0 {
					fmt.Printf("  allow         : %v\n", userConfig.MCPFurlCfg.Allow)
				}
				if len(userConfig.MCPFurlCfg.Disallow) > 0 {
					fmt.Printf("  disallow      : %v\n", userConfig.MCPFurlCfg.Disallow)
				}
			}
			if userConfig.HTTPCfg != nil {
				fmt.Println("[http]")
				if userConfig.HTTPCfg.Addr != nil {
					fmt.Printf("  addr.     : %s\n", *userConfig.HTTPCfg.Addr)
				}
				if userConfig.HTTPCfg.Port != nil {
					fmt.Printf("  port.     : %d\n", *userConfig.HTTPCfg.Port)
				}
				if userConfig.HTTPCfg.MasterKey != nil {
					fmt.Printf("  master_key: ********\n")
				}
			}
			if userConfig.GoogleCustomCfg != nil {
				fmt.Println("[google_custom]")
				if userConfig.GoogleCustomCfg.Cx != nil {
					fmt.Printf("  cx : %s\n", *userConfig.GoogleCustomCfg.Cx)
				}
				if userConfig.GoogleCustomCfg.Key != nil {
					fmt.Printf("  key: %s\n", *userConfig.GoogleCustomCfg.Key)
				}
			}
			if userConfig.CacheCfg != nil {
				fmt.Println("[cache]")
				if userConfig.CacheCfg.SearchDB != nil {
					fmt.Printf("  db_path : %s\n", *userConfig.CacheCfg.SearchDB)
				}
				if userConfig.CacheCfg.Expires != nil {
					fmt.Printf("  expires: %s\n", *userConfig.CacheCfg.Expires)
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
		// fmt.Printf("web_driver_log : %s\n", webDriverLog)
		fmt.Printf("use_pandoc     : %t\n", usePandoc)
		fmt.Printf("verbose        : %t\n", verbose)
		fmt.Printf("search_engine  : %s\n", searchEngine)
		fmt.Printf("search_cache   : %s\n", searchCachePath)
		fmt.Printf("cache_expires  : %s\n", searchCacheExpiresStr)
		fmt.Printf("google_cx      : %s\n", googleCx)
		fmt.Printf("google_key     : %s\n", googleKey)
		if userConfig.HTTPCfg.MasterKey != nil && *userConfig.HTTPCfg.MasterKey != "" {
			fmt.Printf("master_key     : ********\n")
		} else {
			fmt.Printf("master_key     : \n")
		}
		fmt.Printf("mcp_addr       : %s\n", mcpAddr)
		fmt.Printf("mcp_port       : %d\n", mcpPort)
		fmt.Printf("image_max_bytes: %d\n", fetchurl.DefaultMaxDownloadBytes)
		fmt.Printf("fetcher_allow  : %v\n", fetcherAllow)
		fmt.Printf("fetcher_deny   : %v\n", fetcherDeny)
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

	configUsage := "Path to config file (TOML). You can also set MCPFURL_CONFIG."
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
