package cmd

import (
	"log"
	"os"
	"strings"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

type appConfig struct {
	MCPFurlCfg      *MCPFurlConfig       `toml:"mcpfurl"`
	HTTPCfg         *MCPHTTPServerConfig `toml:"http"`
	GoogleCustomCfg *GoogleCustomConfig  `toml:"google_custom"`
	CacheCfg        *CacheConfig         `toml:"cache"`
	SummaryLLMCfg   *SummaryLLMConfig    `toml:"summarize"`
}

type MCPFurlConfig struct {
	// WebDriverPort *int    `toml:"web_driver_port"`
	// WebDriverPath *string `toml:"web_driver_path"`
	// WebDriverLog  *string `toml:"web_driver_log"`
	UsePandoc      *bool    `toml:"use_pandoc"`
	SearchEngine   *string  `toml:"search_engine"`
	Verbose        *bool    `toml:"verbose"`
	FetchDesc      *string  `toml:"fetch_tool_desc"`
	SearchDesc     *string  `toml:"search_tool_desc"`
	ImageDesc      *string  `toml:"image_tool_desc"`
	SummaryDesc    *string  `toml:"summary_tool_desc"`
	DisableFetch   *bool    `toml:"disable_fetch"`
	DisableImage   *bool    `toml:"disable_image"`
	DisableSearch  *bool    `toml:"disable_search"`
	DisableSummary *bool    `toml:"disable_summary"`
	Allow          []string `toml:"allow"`
	Deny           []string `toml:"deny"`

	// Note: these are only configurable through config.toml, no cmdline arguments
	SelectorCfg []UrlSelectorConfig `toml:"selectors"`
}

type UrlSelectorConfig struct {
	Url      *string `toml:"url"`
	Selector *string `toml:"selector"`
}
type MCPHTTPServerConfig struct {
	Addr      *string `toml:"addr"`
	Port      *int    `toml:"port"`
	MasterKey *string `toml:"master_key"`
}

type SummaryLLMConfig struct {
	BaseURL *string `toml:"base_url"`
	ApiKey  *string `toml:"api_key"`
	Model   *string `toml:"model"`
	Short   *bool   `toml:"short"`
}

type GoogleCustomConfig struct {
	Cx  *string `toml:"cx"`
	Key *string `toml:"key"`
}

type CacheConfig struct {
	SearchDB *string `toml:"db_path"`
	Expires  *string `toml:"expires"`
}

var configFilePath string
var userConfig *appConfig // this holds the active merged config (useful for debugging)

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
	applySummaryConfig(cmd)
	if userConfig == nil || userConfig.MCPFurlCfg == nil {
		return
	}
	applyCommonConfig(cmd, userConfig.MCPFurlCfg)
}

func applyMCPHTTPConfig(cmd *cobra.Command) {
	applyCacheConfig(cmd)
	applyGoogleCustomConfig(cmd)
	applySummaryConfig(cmd)
	if userConfig == nil {
		applyHTTPMasterKeyEnv(cmd)
		return
	}

	if cfg := userConfig.HTTPCfg; cfg != nil {
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

func applyCommonConfig(cmd *cobra.Command, cfg *MCPFurlConfig) {
	if cfg == nil {
		return
	}

	// if cfg.WebDriverPort != nil && !cmd.Flags().Changed("wd-port") {
	// 	webDriverPort = *cfg.WebDriverPort
	// }
	// if cfg.WebDriverPath != nil && !cmd.Flags().Changed("wd-path") {
	// 	webDriverPath = *cfg.WebDriverPath
	// }
	// if cfg.WebDriverLog != nil && !cmd.Flags().Changed("wd-log") {
	// 	webDriverLog = *cfg.WebDriverLog
	// }
	if cfg.UsePandoc != nil && !cmd.Flags().Changed("pandoc") {
		usePandoc = *cfg.UsePandoc
	}
	if cfg.DisableFetch != nil && !cmd.Flags().Changed("disable-fetch") {
		disableFetch = *cfg.DisableFetch
	}
	if cfg.DisableImage != nil && !cmd.Flags().Changed("disable-image") {
		disableImage = *cfg.DisableImage
	}
	if cfg.DisableSearch != nil && !cmd.Flags().Changed("disable-search") {
		disableSearch = *cfg.DisableSearch
	}
	if cfg.DisableSummary != nil && !cmd.Flags().Changed("disable-summary") {
		disableSummary = *cfg.DisableSummary
	}
	if cfg.SearchEngine != nil && !cmd.Flags().Changed("search-engine") {
		searchEngine = *cfg.SearchEngine
	}
	if cfg.Verbose != nil && !cmd.Flags().Changed("verbose") {
		verbose = *cfg.Verbose
	}
	if cfg.FetchDesc != nil {
		defaultFetchDesc = *cfg.FetchDesc
	}
	if cfg.ImageDesc != nil {
		defaultImageDesc = *cfg.ImageDesc
	}
	if cfg.SearchDesc != nil {
		defaultSearchDesc = *cfg.SearchDesc
	}
	if cfg.SummaryDesc != nil {
		defaultSummaryDesc = *cfg.SummaryDesc
	}
	if len(cfg.Allow) > 0 && !cmd.Flags().Changed("allow") {
		httpAllowGlobs = normalizePatterns(cfg.Allow)
	}
	if len(cfg.Deny) > 0 && !cmd.Flags().Changed("disallow") {
		httpDenyGlobs = normalizePatterns(cfg.Deny)
	}

	if len(cfg.SelectorCfg) > 0 {
		selectors = []fetchurl.UrlSelector{}
		for _, s := range cfg.SelectorCfg {
			if s.Url != nil && s.Selector != nil {
				selectors = append(selectors, fetchurl.UrlSelector{Url: *s.Url, Selector: *s.Selector})
			}
		}
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

func applySummaryConfig(cmd *cobra.Command) {
	if userConfig == nil || userConfig.GoogleCustomCfg == nil {
		return
	}
	cfg := userConfig.SummaryLLMCfg
	if cfg != nil {
		if cfg.ApiKey != nil && !cmd.Flags().Changed("llm-api-key") {
			summaryAPIKey = *cfg.ApiKey

		}
		if cfg.BaseURL != nil && !cmd.Flags().Changed("llm-baseurl") {
			summaryBaseURL = *cfg.BaseURL
		}
		if cfg.Model != nil && !cmd.Flags().Changed("llm-model") {
			summaryLLMModel = *cfg.Model
		}
		if cfg.Short != nil && !cmd.Flags().Changed("llm-short") {
			summaryShort = *cfg.Short
		}

		if summaryAPIKey == "" {
			if env := os.Getenv("LLM_API_KEY"); env != "" {
				summaryAPIKey = env
			}
		}
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

func normalizePatterns(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// func fetcherAllowPatterns() []string {
// 	return clonePatterns(fetcherAllow)
// }

// func fetcherDenyPatterns() []string {
// 	return clonePatterns(fetcherDeny)
// }

// func clonePatterns(src []string) []string {
// 	if len(src) == 0 {
// 		return nil
// 	}
// 	out := make([]string, len(src))
// 	copy(out, src)
// 	return out
// }
