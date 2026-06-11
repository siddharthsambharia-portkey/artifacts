package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/server"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Artifact server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			if serverVersion := server.ServerVersion; ver != serverVersion {
				logger.Warn("CLI/server version mismatch", "cli", ver, "server", serverVersion)
			}
			srv, err := server.New(cfg, logger)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				srv.Shutdown(context.Background())
			}()
			return srv.ListenAndServe()
		},
	}
}

func devCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dev",
		Short: "Start local dev server with live reload",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				cfg = config.DefaultDev()
			}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			logger.Info("dev mode", "url", "http://<site>.localhost:8443", "identity", "dev@localhost")
			srv, err := server.New(cfg, logger)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				srv.Shutdown(context.Background())
			}()
			return srv.ListenAndServe()
		},
	}
}
