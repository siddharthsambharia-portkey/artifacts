package governance

import (
	"context"
	"testing"

	"github.com/siddharthsambharia-portkey/artifacts/internal/auth"
	"github.com/siddharthsambharia-portkey/artifacts/internal/config"
	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

func TestCanDeploy(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		mode     string
		user     *auth.User
		existing *db.SiteRecord
		wantErr  bool
	}{
		{
			name:     "trust mode allows non-owner",
			mode:     "trust",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  false,
		},
		{
			name:     "governed new site nil existing",
			mode:     "governed",
			user:     &auth.User{Email: "b@co"},
			existing: nil,
			wantErr:  false,
		},
		{
			name:     "governed empty owner",
			mode:     "governed",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "", Visibility: "private"},
			wantErr:  false,
		},
		{
			name:     "governed owner match",
			mode:     "governed",
			user:     &auth.User{Email: "a@co"},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  false,
		},
		{
			name:     "governed owner mismatch denied",
			mode:     "governed",
			user:     &auth.User{Email: "b@co", Groups: []string{"employees"}},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  true,
		},
		{
			name:     "governed admin can deploy",
			mode:     "governed",
			user:     &auth.User{Email: "b@co", Groups: []string{"admins"}},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultDev()
			cfg.Governance.Mode = tt.mode
			g := New(cfg)
			err := g.CanDeploy(ctx, tt.user, "demo", tt.existing)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CanDeploy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanWriteDB(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		mode     string
		user     *auth.User
		existing *db.SiteRecord
		wantErr  bool
	}{
		{
			name:     "trust mode allows write",
			mode:     "trust",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "a@co"},
			wantErr:  false,
		},
		{
			name:     "governed non-owner denied",
			mode:     "governed",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "a@co"},
			wantErr:  true,
		},
		{
			name:     "governed admin can write",
			mode:     "governed",
			user:     &auth.User{Email: "b@co", Groups: []string{"admins"}},
			existing: &db.SiteRecord{Owner: "a@co"},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultDev()
			cfg.Governance.Mode = tt.mode
			g := New(cfg)
			err := g.CanWriteDB(ctx, tt.user, "demo", tt.existing)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CanWriteDB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanReadSite(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		mode     string
		user     *auth.User
		existing *db.SiteRecord
		wantErr  bool
	}{
		{
			name:     "trust mode allows private read",
			mode:     "trust",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  false,
		},
		{
			name:     "governed nil existing allowed",
			mode:     "governed",
			user:     &auth.User{Email: "b@co"},
			existing: nil,
			wantErr:  false,
		},
		{
			name:     "governed public site allowed",
			mode:     "governed",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "public"},
			wantErr:  false,
		},
		{
			name:     "governed owner can read private",
			mode:     "governed",
			user:     &auth.User{Email: "a@co"},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  false,
		},
		{
			name:     "governed admin can read private site",
			mode:     "governed",
			user:     &auth.User{Email: "b@co", Groups: []string{"admins"}},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  false,
		},
		{
			name:     "governed nil user private denied",
			mode:     "governed",
			user:     nil,
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  true,
		},
		{
			name:     "governed nil user public allowed",
			mode:     "governed",
			user:     nil,
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "public"},
			wantErr:  false,
		},
		{
			name:     "governed non-owner private denied",
			mode:     "governed",
			user:     &auth.User{Email: "b@co"},
			existing: &db.SiteRecord{Owner: "a@co", Visibility: "private"},
			wantErr:  true,
		},
		{
			name: "governed group site member allowed",
			mode: "governed",
			user: &auth.User{Email: "b@co", Groups: []string{"employees", "hr-team"}},
			existing: &db.SiteRecord{
				Owner: "a@co", Visibility: "group", VisibilityGroups: []string{"hr-team"},
			},
			wantErr: false,
		},
		{
			name: "governed group site non-member denied",
			mode: "governed",
			user: &auth.User{Email: "b@co", Groups: []string{"employees"}},
			existing: &db.SiteRecord{
				Owner: "a@co", Visibility: "group", VisibilityGroups: []string{"hr-team"},
			},
			wantErr: true,
		},
		{
			name: "governed group site owner allowed without group",
			mode: "governed",
			user: &auth.User{Email: "a@co", Groups: []string{"employees"}},
			existing: &db.SiteRecord{
				Owner: "a@co", Visibility: "group", VisibilityGroups: []string{"hr-team"},
			},
			wantErr: false,
		},
		{
			name: "governed group site admin allowed without group",
			mode: "governed",
			user: &auth.User{Email: "b@co", Groups: []string{"admins"}},
			existing: &db.SiteRecord{
				Owner: "a@co", Visibility: "group", VisibilityGroups: []string{"hr-team"},
			},
			wantErr: false,
		},
		{
			name: "governed group site empty groups denied",
			mode: "governed",
			user: &auth.User{Email: "b@co", Groups: []string{"employees"}},
			existing: &db.SiteRecord{
				Owner: "a@co", Visibility: "group", VisibilityGroups: []string{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultDev()
			cfg.Governance.Mode = tt.mode
			g := New(cfg)
			err := g.CanReadSite(ctx, tt.user, "demo", tt.existing)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CanReadSite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	cfg := config.DefaultDev()
	g := New(cfg)
	tests := []struct {
		name string
		user *auth.User
		want bool
	}{
		{"no groups", &auth.User{Email: "u@co"}, false},
		{"employees only", &auth.User{Email: "u@co", Groups: []string{"employees"}}, false},
		{"admins group", &auth.User{Email: "u@co", Groups: []string{"admins"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.IsAdmin(tt.user); got != tt.want {
				t.Fatalf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}
