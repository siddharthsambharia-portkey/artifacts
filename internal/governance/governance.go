package governance

import (
	"context"
	"fmt"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

type Governor struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Governor {
	return &Governor{cfg: cfg}
}

func (g *Governor) IsTrustMode() bool {
	return g.cfg.Governance.Mode == "trust"
}

func (g *Governor) CanDeploy(ctx context.Context, user *auth.User, site string, existing *db.SiteRecord) error {
	if g.IsTrustMode() {
		return nil
	}
	if existing == nil {
		return nil
	}
	if existing.Owner == "" || existing.Owner == user.Email {
		return nil
	}
	for _, grp := range user.Groups {
		if grp == "admins" {
			return nil
		}
	}
	return fmt.Errorf("governed mode: only the site owner (%s) or admins can redeploy %q — contact them or ask an admin", existing.Owner, site)
}

func (g *Governor) CanWriteDB(ctx context.Context, user *auth.User, site string, existing *db.SiteRecord) error {
	if g.IsTrustMode() {
		return nil
	}
	return g.CanDeploy(ctx, user, site, existing)
}

func (g *Governor) CanReadSite(ctx context.Context, user *auth.User, site string, existing *db.SiteRecord) error {
	if g.IsTrustMode() || existing == nil {
		return nil
	}
	if existing.Visibility == "public" {
		return nil
	}
	if existing.Owner == user.Email {
		return nil
	}
	return fmt.Errorf("governed mode: you do not have access to site %q", site)
}

func (g *Governor) IsAdmin(user *auth.User) bool {
	for _, g := range user.Groups {
		if g == "admins" {
			return true
		}
	}
	return false
}
