package cmd

import (
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

type appConfig struct {
	MCPFurlCfg      *MCPCommandConfig     `toml:"mcpfurl"`
	HTTPCfg         *MCPHTTPCommandConfig `toml:"http"`
	GoogleCustomCfg *GoogleCustomConfig   `toml:"google_custom"`
	CacheCfg        *CacheConfig          `toml:"cache"`
}

type MCPCommandConfig struct {
	WebDriverPort *int    `toml:"web_driver_port"`
	WebDriverPath *string `toml:"web_driver_path"`
	// WebDriverLog  *string `toml:"web_driver_log"`
	UsePandoc    *bool   `toml:"use_pandoc"`
	SearchEngine *string `toml:"search_engine"`
	Verbose      *bool   `toml:"verbose"`
}

type MCPHTTPCommandConfig struct {
	MCPCommandConfig
	Addr      *string `toml:"addr"`
	Port      *int    `toml:"port"`
	MasterKey *string `toml:"master_key"`
}

type GoogleCustomConfig struct {
	Cx  *string `toml:"cx"`
	Key *string `toml:"key"`
}

type CacheConfig struct {
	SearchDB *string `toml:"db_path"`
	Expires  *string `toml:"expires"`
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
	if err := toml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("error parsing config file %s: %v", path, err)
	}

	// Keep this commented code as a reference for troubleshooting config issues
	///////////////
	// if dump, err := toml.Marshal(cfg); err == nil {
	// 	fmt.Println("===== loaded config =====")
	// 	fmt.Printf("%s\n", dump)
	// 	fmt.Println("=========================")
	// }
	userConfig = &cfg
}

func applyMCPConfig(cmd *cobra.Command) {
	applyCacheConfig(cmd)
	applyGoogleCustomConfig(cmd)
	if userConfig == nil || userConfig.MCPFurlCfg == nil {
		return
	}
	applyCommonConfig(cmd, userConfig.MCPFurlCfg)
}

func applyMCPHTTPConfig(cmd *cobra.Command) {
	applyCacheConfig(cmd)
	applyGoogleCustomConfig(cmd)
	if userConfig == nil {
		applyHTTPMasterKeyEnv(cmd)
		return
	}

	if cfg := userConfig.HTTPCfg; cfg != nil {
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

	if userConfig.MCPFurlCfg != nil {
		applyCommonConfig(cmd, userConfig.MCPFurlCfg)
	}

	// ENV loading happens after loading the config from a file
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
	// if cfg.WebDriverLog != nil && !cmd.Flags().Changed("wd-log") {
	// 	webDriverLog = *cfg.WebDriverLog
	// }
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
	if userConfig == nil || userConfig.GoogleCustomCfg == nil {
		return
	}
	cfg := userConfig.GoogleCustomCfg

	if cfg.Cx != nil && !cmd.Flags().Changed("google-cx") {
		googleCx = *cfg.Cx
	}
	if cfg.Key != nil && !cmd.Flags().Changed("google-key") {
		googleKey = *cfg.Key
	}
}

func applyCacheConfig(cmd *cobra.Command) {
	if userConfig == nil || userConfig.CacheCfg == nil {
		return
	}
	cfg := userConfig.CacheCfg

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
