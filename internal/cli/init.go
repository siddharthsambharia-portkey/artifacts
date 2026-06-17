package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [name]",
		Short: "Create a new Artifact site with starter files and agent skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "my-site"
			if len(args) > 0 {
				name = args[0]
			}
			if err := os.MkdirAll(name, 0755); err != nil {
				return err
			}
			indexHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + name + `</title>
  <script src="/artifact.js"></script>
</head>
<body>
  <h1>Welcome to ` + name + `</h1>
  <p>Signed in as <span id="user"></span></p>
  <script>
    artifact.ready().then(() => {
      document.getElementById('user').textContent = artifact.me.name;
    });
  </script>
</body>
</html>
`
			if err := os.WriteFile(filepath.Join(name, "index.html"), []byte(indexHTML), 0644); err != nil {
				return err
			}
			artifactJSON := fmt.Sprintf(`{"site":"%s"}`, name)
			if err := os.WriteFile(filepath.Join(name, "artifact.json"), []byte(artifactJSON), 0644); err != nil {
				return err
			}
			for _, skill := range []string{"AGENTS.md", "CLAUDE.md"} {
				data, err := os.ReadFile(filepath.Join("skills", skill))
				if err != nil {
					data = []byte("# Artifact SDK\n\nSee https://github.com/siddharthsambharia-portkey/artifacts for docs.\n")
				}
				_ = os.WriteFile(filepath.Join(name, skill), data, 0644)
			}
			fmt.Printf("✓ Created site in ./%s\n", name)
			fmt.Printf("  Run: cd %s && artifact deploy\n", name)
			return nil
		},
	}
}

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate via OIDC device code flow",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Dev mode: you are already signed in as dev@localhost when using artifact dev.")
			fmt.Println("Production: visit your Artifact domain and sign in via SSO.")
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List deployed sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := configOrDev()
			database, err := db.Open(cfg)
			if err != nil {
				return err
			}
			sites, err := database.ListSites(context.Background())
			if err != nil {
				return err
			}
			if len(sites) == 0 {
				fmt.Println("No sites deployed yet. Run artifact init && artifact deploy.")
				return nil
			}
			for _, s := range sites {
				fmt.Printf("%-20s  %s  (%s, %d bytes)\n", s.Name, s.DeployedAt.Format("2006-01-02 15:04"), s.DeployedBy, s.SizeBytes)
			}
			return nil
		},
	}
}

func openCmd() *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open a site in the browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := configOrDev()
			if site == "" && len(args) > 0 {
				site = args[0]
			}
			if site == "" {
				return fmt.Errorf("specify a site name: artifact open --site mysite")
			}
			url := fmt.Sprintf("http://%s.localhost%s", site, cfg.Listen)
			if cfg.Domain != "localhost" {
				url = fmt.Sprintf("https://%s.%s", site, cfg.Domain)
			}
			fmt.Printf("Open: %s\n", url)
			return nil
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "site name")
	return cmd
}

func logsCmd() *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View audit logs for a site",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := configOrDev()
			database, err := db.Open(cfg)
			if err != nil {
				return err
			}
			entries, err := database.SearchAudit(context.Background(), site, "", 50)
			if err != nil {
				return err
			}
			for _, e := range entries {
				fmt.Printf("%s  %s  %s  %s  %s\n", e.Timestamp.Format("2006-01-02 15:04:05"), e.UserEmail, e.Site, e.Action, e.Detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "filter by site")
	return cmd
}

func configOrDev() *config.Config {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return config.DefaultDev()
	}
	return cfg
}
