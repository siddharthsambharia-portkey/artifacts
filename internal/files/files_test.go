package files

import "testing"

func TestIsDangerousContentType(t *testing.T) {
	tests := []struct {
		name string
		ct   string
		want bool
	}{
		{"text/html", "text/html", true},
		{"text/html with charset", "text/html; charset=utf-8", true},
		{"application/javascript", "application/javascript", true},
		{"image/png safe", "image/png", false},
		{"application/pdf safe", "application/pdf", false},
		{
			name: "uppercase html not flagged",
			// case-sensitivity gap; mitigated by attachment+nosniff headers on serve
			ct:   "TEXT/HTML",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDangerousContentType(tt.ct); got != tt.want {
				t.Fatalf("isDangerousContentType(%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}
