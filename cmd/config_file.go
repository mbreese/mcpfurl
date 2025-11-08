package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type appConfig struct {
	MCP          *MCPCommandConfig     `yaml:"mcp"`
	HTTP         *MCPHTTPCommandConfig `yaml:"http"`
	GoogleCustom *GoogleCustomConfig   `yaml:"google_custom"`
	Cache        *CacheConfig          `yaml:"cache"`
}

type MCPCommandConfig struct {
	WebDriverPort *int    `yaml:"web_driver_port"`
	WebDriverPath *string `yaml:"web_driver_path"`
	UsePandoc     *bool   `yaml:"use_pandoc"`
	SearchEngine  *string `yaml:"search_engine"`
	SearchCache   *string `yaml:"search_cache"`
	Verbose       *bool   `yaml:"verbose"`
}

type MCPHTTPCommandConfig struct {
	MCPCommandConfig
	Addr      *string `yaml:"addr"`
	Port      *int    `yaml:"port"`
	MasterKey *string `yaml:"master_key"`
}

type GoogleCustomConfig struct {
	Cx  *string `yaml:"cx"`
	Key *string `yaml:"key"`
}

type CacheConfig struct {
	SearchDB *string `yaml:"db_path"`
	Expires  *string `yaml:"expires"`
}

var (
	configFilePath string
	userConfig     *appConfig
)

func loadConfigFile() {
	path := configFilePath
	explicit := path != ""

	if path == "" {
		if envPath := os.Getenv("MCPFURL_CONFIG"); envPath != "" {
			path = envPath
			explicit = true
		}
	}

	if path == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicit {
			log.Fatalf("error reading config file %s: %v", path, err)
		}
		return
	}
	var cfg appConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("error parsing config file %s: %v", path, err)
	}

	// Keep this commented code as a reference for troubleshooting config issues
	///////////////
	// if dump, err := yaml.Marshal(cfg); err == nil {
	// 	fmt.Println("===== loaded config =====")
	// 	fmt.Printf("%s\n", dump)
	// 	fmt.Println("=========================")
	// }
	userConfig = &cfg
}

func applyMCPConfig(cmd *cobra.Command) {
	applyCacheConfig(cmd)
	applyGoogleCustomConfig(cmd)
	if userConfig == nil || userConfig.MCP == nil {
		return
	}
	applyCommonConfig(cmd, userConfig.MCP)
}

func applyMCPHTTPConfig(cmd *cobra.Command) {
	applyCacheConfig(cmd)
	applyGoogleCustomConfig(cmd)
	if userConfig == nil {
		applyHTTPMasterKeyEnv(cmd)
		return
	}

	if cfg := userConfig.HTTP; cfg != nil {
		applyCommonConfig(cmd, &cfg.MCPCommandConfig)
		if cfg.Addr != nil && !cmd.Flags().Changed("addr") {
			mcpAddr = *cfg.Addr
		}
		if cfg.Port != nil && !cmd.Flags().Changed("port") {
			mcpPort = *cfg.Port
		}
		if cfg.MasterKey != nil && !cmd.Flags().Changed("master-key") {
			masterKey = *cfg.MasterKey
		}
		applyHTTPMasterKeyEnv(cmd)
		return
	}

	if userConfig.MCP != nil {
		applyCommonConfig(cmd, userConfig.MCP)
	}

	applyHTTPMasterKeyEnv(cmd)
}

func applyCommonConfig(cmd *cobra.Command, cfg *MCPCommandConfig) {
	if cfg == nil {
		return
	}

	if cfg.WebDriverPort != nil && !cmd.Flags().Changed("wd-port") {
		webDriverPort = *cfg.WebDriverPort
	}
	if cfg.WebDriverPath != nil && !cmd.Flags().Changed("wd-path") {
		webDriverPath = *cfg.WebDriverPath
	}
	if cfg.UsePandoc != nil && !cmd.Flags().Changed("pandoc") {
		usePandoc = *cfg.UsePandoc
	}
	if cfg.SearchEngine != nil && !cmd.Flags().Changed("search-engine") {
		searchEngine = *cfg.SearchEngine
	}
	if cfg.Verbose != nil && !cmd.Flags().Changed("verbose") {
		verbose = *cfg.Verbose
	}
}

func applyGoogleCustomConfig(cmd *cobra.Command) {
	if userConfig == nil || userConfig.GoogleCustom == nil {
		return
	}
	cfg := userConfig.GoogleCustom

	if cfg.Cx != nil && !cmd.Flags().Changed("google-cx") {
		googleCx = *cfg.Cx
	}
	if cfg.Key != nil && !cmd.Flags().Changed("google-key") {
		googleKey = *cfg.Key
	}
}

func applyCacheConfig(cmd *cobra.Command) {
	if userConfig == nil || userConfig.Cache == nil {
		return
	}
	cfg := userConfig.Cache

	if cfg.SearchDB != nil && !cmd.Flags().Changed("search-cache") {
		searchCachePath = *cfg.SearchDB
	}
	if cfg.Expires != nil && !cmd.Flags().Changed("search-expires") {
		searchCacheExpiresStr = *cfg.Expires
	}
}

func applyHTTPMasterKeyEnv(cmd *cobra.Command) {
	if cmd.Flags().Changed("master-key") {
		return
	}
	if env := os.Getenv("MCPFETCH_MASTER_KEY"); env != "" {
		masterKey = env
	}
}
