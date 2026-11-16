package domain

import (
	"testing"
	"time"
)

func TestLinkIsExpired(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{name: "no expiration", expiresAt: nil, want: false},
		{name: "past expiration", expiresAt: &past, want: true},
		{name: "future expiration", expiresAt: &future, want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			link := &Link{ExpiresAt: tc.expiresAt}
			if got := link.IsExpired(); got != tc.want {
				t.Fatalf("IsExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}

