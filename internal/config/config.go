package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Branding    Branding    `yaml:"branding"`
	Domain      string      `yaml:"domain"`
	Listen        string      `yaml:"listen"`
	TLS           TLS         `yaml:"tls"`
	Auth          Auth        `yaml:"auth"`
	Storage       Storage     `yaml:"storage"`
	Database      Database    `yaml:"database"`
	AI            AI          `yaml:"ai"`
	Warehouse     Warehouse   `yaml:"warehouse"`
	Notify        Notify      `yaml:"notify"`
	Governance    Governance  `yaml:"governance"`
	DataDir       string      `yaml:"data_dir"`
}

type Branding struct {
	Name string `yaml:"name"`
	Logo string `yaml:"logo"`
}

type TLS struct {
	Mode string `yaml:"mode"`
}

type Auth struct {
	Mode        string      `yaml:"mode"`
	OIDC        OIDC        `yaml:"oidc"`
	HeaderTrust HeaderTrust `yaml:"header_trust"`
}

type OIDC struct {
	Issuer            string `yaml:"issuer"`
	ClientID          string `yaml:"client_id"`
	ClientSecretEnv   string `yaml:"client_secret_env"`
	GroupsClaim       string `yaml:"groups_claim"`
}

type HeaderTrust struct {
	EmailHeader     string `yaml:"email_header"`
	NameHeader      string `yaml:"name_header"`
	ProxySecretEnv  string `yaml:"proxy_secret_env"`
}

type Storage struct {
	Driver   string `yaml:"driver"`
	Bucket   string `yaml:"bucket"`
	Endpoint string `yaml:"endpoint"`
	Path     string `yaml:"path"`
}

type Database struct {
	Driver string `yaml:"driver"`
	URLEnv string `yaml:"url_env"`
	URL    string `yaml:"url"`
}

type AI struct {
	UpstreamURL    string   `yaml:"upstream_url"`
	APIKeyEnv      string   `yaml:"api_key_env"`
	ImageModel     string   `yaml:"image_model"`
	ModelsAllowlist []string `yaml:"models_allowlist"`
}

type Warehouse struct {
	Driver          string   `yaml:"driver"`
	CredentialsEnv  string   `yaml:"credentials_env"`
	AllowedDatasets []string `yaml:"allowed_datasets"`
	RowLimit        int      `yaml:"row_limit"`
}

type Notify struct {
	Slack SlackNotify `yaml:"slack"`
}

type SlackNotify struct {
	Mode              string   `yaml:"mode"`
	SecretEnv         string   `yaml:"secret_env"`
	ChannelAllowlist  []string `yaml:"channel_allowlist"`
}

type Governance struct {
	Mode   string `yaml:"mode"`
	Quotas Quotas `yaml:"quotas"`
}

type Quotas struct {
	SiteMaxMB                  int `yaml:"site_max_mb"`
	DBMaxDocsPerSite           int `yaml:"db_max_docs_per_site"`
	UploadMaxMB                int `yaml:"upload_max_mb"`
	AIDailyTokensPerUser       int `yaml:"ai_daily_tokens_per_user"`
	WarehouseDailyQueriesPerUser int `yaml:"warehouse_daily_queries_per_user"`
}

func DefaultDev() *Config {
	return &Config{
		Branding: Branding{Name: "Artifact"},
		Domain:   "localhost",
		Listen:   ":8443",
		TLS:      TLS{Mode: "off"},
		Auth:     Auth{Mode: "dev"},
		Storage: Storage{
			Driver: "local",
			Path:   ".artifact-data",
		},
		Database: Database{
			Driver: "sqlite",
			URL:    ".artifact-data/artifact.db",
		},
		AI: AI{
			UpstreamURL: "",
			APIKeyEnv:   "ARTIFACT_AI_KEY",
		},
		Warehouse: Warehouse{
			Driver:   "none",
			RowLimit: 10000,
		},
		Notify: Notify{
			Slack: SlackNotify{Mode: "off"},
		},
		Governance: Governance{
			Mode: "trust",
			Quotas: Quotas{
				SiteMaxMB:                    500,
				DBMaxDocsPerSite:             100000,
				UploadMaxMB:                  50,
				AIDailyTokensPerUser:         0,
				WarehouseDailyQueriesPerUser: 200,
			},
		},
		DataDir: ".artifact-data",
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultDev()
	if path == "" {
		path = os.Getenv("ARTIFACT_CONFIG")
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ARTIFACT_DOMAIN"); v != "" {
		cfg.Domain = v
	}
	if v := os.Getenv("ARTIFACT_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("ARTIFACT_AUTH_MODE"); v != "" {
		cfg.Auth.Mode = v
	}
	if v := os.Getenv("ARTIFACT_STORAGE_DRIVER"); v != "" {
		cfg.Storage.Driver = v
	}
	if v := os.Getenv("ARTIFACT_DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("ARTIFACT_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("ARTIFACT_GOVERNANCE_MODE"); v != "" {
		cfg.Governance.Mode = v
	}
}

func (c *Config) Validate() error {
	switch c.Auth.Mode {
	case "dev", "oidc", "header-trust":
	default:
		return fmt.Errorf("invalid auth.mode %q: use dev, oidc, or header-trust", c.Auth.Mode)
	}
	if c.Auth.Mode == "header-trust" {
		secret := os.Getenv(c.Auth.HeaderTrust.ProxySecretEnv)
		if secret == "" && c.Auth.HeaderTrust.ProxySecretEnv != "" {
			return fmt.Errorf("header-trust mode requires %s to be set — configure your identity proxy shared secret", c.Auth.HeaderTrust.ProxySecretEnv)
		}
		if c.Auth.HeaderTrust.ProxySecretEnv == "" {
			return fmt.Errorf("header-trust mode requires proxy_secret_env in config — Artifact refuses to boot without proxy authentication")
		}
	}
	if c.Governance.Mode != "trust" && c.Governance.Mode != "governed" {
		return fmt.Errorf("invalid governance.mode %q: use trust or governed", c.Governance.Mode)
	}
	return nil
}

func (c *Config) ApexHost() string {
	return c.Domain
}

func (c *Config) SiteFromHost(host string) string {
	host = strings.Split(host, ":")[0]
	domain := c.Domain
	if strings.HasSuffix(host, "."+domain) {
		sub := strings.TrimSuffix(host, "."+domain)
		if sub != "" && sub != "admin" {
			return sub
		}
	}
	if domain == "localhost" {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 && parts[len(parts)-1] == "localhost" {
			sub := parts[0]
			if sub != "admin" && sub != "localhost" {
				return sub
			}
		}
	}
	return ""
}

func (c *Config) IsAdminHost(host string) bool {
	host = strings.Split(host, ":")[0]
	return host == "admin."+c.Domain || (c.Domain == "localhost" && strings.HasPrefix(host, "admin."))
}

func (c *Config) IsApexHost(host string) bool {
	host = strings.Split(host, ":")[0]
	return host == c.Domain || host == "localhost"
}
