package render

import (
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Preview is the week downsampled into buckets for the visual outputs. The canonical output is never downsampled.
type Preview struct {
	// Days holds, per day, the tier id shown in each bucket: 1440/Size entries.
	Days [][]string
	// Size is the bucket length in minutes; it divides 60.
	Size int
	// Mixed counts buckets that contain more than one tier; they show the majority tier, so the preview is
	// approximate.
	Mixed int
}

// segment is a stretch of equal buckets in one day, in minutes [from, to).
type segment struct {
	tier string
	from int
	to   int
}

// Downsample buckets the week. A bucket shows the tier covering most of its minutes; ties go to the tier that comes
// later in the configured tier list.
func Downsample(out *schedule.Output, cfg *config.Config, size int) Preview {
	p := Preview{Size: size}

	for d := range days {
		minuteTiers := make([]string, 0, minutesPerDay)
		for _, r := range dayRuns(out, d) {
			for range endMinutes(r.End) - minutes(r.Start) {
				minuteTiers = append(minuteTiers, r.Tier)
			}
		}

		buckets := make([]string, 0, minutesPerDay/size)
		for start := 0; start < minutesPerDay; start += size {
			tier, mixed := majority(minuteTiers[start:start+size], cfg)
			buckets = append(buckets, tier)

			if mixed {
				p.Mixed++
			}
		}

		p.Days = append(p.Days, buckets)
	}

	return p
}

func majority(tiers []string, cfg *config.Config) (string, bool) {
	counts := map[string]int{}
	for _, t := range tiers {
		counts[t]++
	}

	best, bestCount, bestIndex := "", -1, -1

	for t, n := range counts {
		i, _ := tierInfo(cfg, t)
		if n > bestCount || (n == bestCount && i > bestIndex) {
			best, bestCount, bestIndex = t, n, i
		}
	}

	return best, len(counts) > 1
}

// segments merges equal neighbouring buckets of day d.
func (p Preview) segments(d int) []segment {
	var out []segment

	for i, tier := range p.Days[d] {
		from := i * p.Size
		if n := len(out); n > 0 && out[n-1].tier == tier {
			out[n-1].to = from + p.Size

			continue
		}

		out = append(out, segment{tier: tier, from: from, to: from + p.Size})
	}

	return out
}
