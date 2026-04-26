package main

import (
	"testing"
	"time"
)

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func TestThreeBusinessDaysAgo(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "from Wednesday gives previous Friday",
			now:  date(2026, 4, 22), // Wed
			want: date(2026, 4, 17), // Fri (Wed->Tue->Mon->Fri)
		},
		{
			name: "from Monday gives previous Wednesday",
			now:  date(2026, 4, 27), // Mon
			want: date(2026, 4, 22), // Wed (Mon->Fri->Thu->Wed)
		},
		{
			name: "from Friday gives previous Tuesday",
			now:  date(2026, 4, 24), // Fri
			want: date(2026, 4, 21), // Tue (Thu->Wed->Tue)
		},
		{
			name: "from Sunday skips weekend",
			now:  date(2026, 4, 26), // Sun
			want: date(2026, 4, 22), // Wed (Sat skipped, Fri->Thu->Wed)
		},
		{
			name: "from Saturday skips weekend",
			now:  date(2026, 4, 25), // Sat
			want: date(2026, 4, 22), // Wed (Fri->Thu->Wed)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := threeBusinessDaysAgo(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("threeBusinessDaysAgo(%s) = %s, want %s",
					tc.now.Format("2006-01-02"),
					got.Format("2006-01-02"),
					tc.want.Format("2006-01-02"),
				)
			}
		})
	}
}
