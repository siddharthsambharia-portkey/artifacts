package cli

import (
	"context"
	"fmt"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/governance"
	"github.com/siddharthsambharia-portkey/artifacts/internal/sites"
	"github.com/siddharthsambharia-portkey/artifacts/internal/storage"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
	"github.com/spf13/cobra"
)

func deployCmd() *cobra.Command {
	var siteFlag string
	var yes bool
	cmd := &cobra.Command{
		Use:   "deploy [dir]",
		Short: "Deploy a folder of static files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			cfg, err := config.Load(cfgFile)
			if err != nil {
				cfg = config.DefaultDev()
			}
			store, err := storage.New(cfg)
			if err != nil {
				return err
			}
			database, err := db.Open(cfg)
			if err != nil {
				return err
			}
			_ = database.Migrate(context.Background())
			siteName := siteFlag
			if siteName == "" {
				siteName = sites.SiteNameFromDir(dir)
			}
			existing, _ := database.GetSite(context.Background(), siteName)
			if existing != nil && !yes {
				fmt.Printf("Site %q exists (last deployed by %s). Overwrite? [y/N] ", siteName, existing.DeployedBy)
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("Deploy cancelled.")
					return nil
				}
			}
			cache := sites.NewDeployCache(store, 512)
			deployer := sites.NewDeployer(cfg, store, database, governance.New(cfg), cache)
			url, err := deployer.Deploy(context.Background(), siteName, dir, auth.DevUser)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Deployed %s → %s\n", siteName, url)
			return nil
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site name (default: from artifact.json or folder name)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "overwrite without confirmation")
	return cmd
}
