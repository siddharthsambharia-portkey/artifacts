package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server on stdio for agent integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := configOrDev()
			store, _ := storage.New(cfg)
			database, _ := db.Open(cfg)
			_ = database.Migrate(context.Background())
			cache := sites.NewDeployCache(store, 512)
			deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)
			scanner := bufio.NewScanner(os.Stdin)
			encoder := json.NewEncoder(os.Stdout)
			for scanner.Scan() {
				var req mcpRequest
				if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
					continue
				}
				resp := handleMCPTool(req, cfg, deployer, database)
				encoder.Encode(resp)
			}
			return nil
		},
	}
}

type mcpRequest struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

type mcpResponse struct {
	ID     string `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func handleMCPTool(req mcpRequest, cfg *config.Config, deployer *sites.Deployer, database *db.DB) mcpResponse {
	tool, _ := req.Params["name"].(string)
	switch tool {
	case "deploy_site":
		dir, _ := req.Params["dir"].(string)
		site, _ := req.Params["site"].(string)
		if dir == "" {
			dir = "."
		}
		if site == "" {
			site = sites.SiteNameFromDir(dir)
		}
		url, err := deployer.Deploy(context.Background(), site, dir, auth.DevUser)
		if err != nil {
			return mcpResponse{ID: req.ID, Error: err.Error()}
		}
		return mcpResponse{ID: req.ID, Result: map[string]string{"url": url, "site": site}}
	case "list_sites":
		sites, err := database.ListSites(context.Background())
		if err != nil {
			return mcpResponse{ID: req.ID, Error: err.Error()}
		}
		return mcpResponse{ID: req.ID, Result: sites}
	case "read_logs":
		site, _ := req.Params["site"].(string)
		entries, err := database.SearchAudit(context.Background(), site, "", 50)
		if err != nil {
			return mcpResponse{ID: req.ID, Error: err.Error()}
		}
		return mcpResponse{ID: req.ID, Result: entries}
	case "query_db":
		site, _ := req.Params["site"].(string)
		collection, _ := req.Params["collection"].(string)
		docs, err := database.ListDocuments(context.Background(), site, collection, 50, true)
		if err != nil {
			return mcpResponse{ID: req.ID, Error: err.Error()}
		}
		return mcpResponse{ID: req.ID, Result: docs}
	default:
		return mcpResponse{ID: req.ID, Error: fmt.Sprintf("unknown tool %q: use deploy_site, list_sites, read_logs, or query_db", tool)}
	}
}
