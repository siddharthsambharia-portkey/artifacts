package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/siddharthsambharia-portkey/artifacts/internal/db"
)

type SessionStore struct {
	db *db.DB
}

func NewSessionStore(database *db.DB) *SessionStore {
	return &SessionStore{db: database}
}

func (s *SessionStore) Create(ctx context.Context, user *User, ttl time.Duration) (string, error) {
	id := randomSessionID()
	expires := time.Now().Add(ttl)
	err := s.db.InsertSession(ctx, id, user.Email, user.Name, groupsJSON(user.Groups), expires)
	return id, err
}

func (s *SessionStore) Get(ctx context.Context, id string) (*User, error) {
	email, name, groupsRaw, expires, err := s.db.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		_ = s.db.DeleteSession(ctx, id)
		return nil, fmt.Errorf("session expired")
	}
	return &User{Email: email, Name: name, Groups: parseGroups(groupsRaw)}, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	return s.db.DeleteSession(ctx, id)
}

func randomSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func groupsJSON(groups []string) string {
	if len(groups) == 0 {
		return "[]"
	}
	out := "["
	for i, g := range groups {
		if i > 0 {
			out += ","
		}
		out += `"` + g + `"`
	}
	return out + "]"
}

func parseGroups(raw string) []string {
	var groups []string
	// minimal JSON array parse
	raw = trim(raw, "[]")
	if raw == "" {
		return groups
	}
	for _, p := range splitComma(raw) {
		groups = append(groups, trim(p, `"`))
	}
	return groups
}

func trim(s, cut string) string {
	for len(s) > 0 && (s[0:1] == cut || s[len(s)-1:] == cut) {
		if s[0:1] == cut {
			s = s[1:]
		}
		if len(s) > 0 && s[len(s)-1:] == cut {
			s = s[:len(s)-1]
		}
	}
	return s
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	inQuote := false
	for _, c := range s {
		if c == '"' {
			inQuote = !inQuote
		}
		if c == ',' && !inQuote {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
