package schedule

import (
	"strconv"
	"testing"
)

func TestCompress(t *testing.T) {
	t.Parallel()

	name := func(i int) string { return sundayFirst[i] }

	tests := []struct {
		posix string
		aws   string
		days  []int
	}{
		{days: []int{1, 2, 3, 4, 5}, posix: "1-5", aws: "MON-FRI"},
		{days: []int{0, 6}, posix: "0,6", aws: "SUN,SAT"},
		{days: []int{0, 1, 5, 6}, posix: "0-1,5-6", aws: "SUN-MON,FRI-SAT"},
		{days: []int{3}, posix: "3", aws: "WED"},
		{days: []int{1, 3, 5}, posix: "1,3,5", aws: "MON,WED,FRI"},
		{days: []int{0, 1, 2, 3, 4, 5, 6}, posix: "*", aws: "*"},
	}

	for _, tt := range tests {
		if got := compress(tt.days, strconv.Itoa); got != tt.posix {
			t.Errorf("compress(%v) posix = %q, want %q", tt.days, got, tt.posix)
		}

		if got := compress(tt.days, name); got != tt.aws {
			t.Errorf("compress(%v) aws = %q, want %q", tt.days, got, tt.aws)
		}
	}
}
