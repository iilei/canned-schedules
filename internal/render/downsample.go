package render

import (
	"github.com/iilei/canned-schedules/internal/config"
	"github.com/iilei/canned-schedules/internal/schedule"
)

// Preview is the week downsampled into buckets for the visual outputs. The canonical output is never downsampled.
type Preview struct {
	// Days holds, per day, the state id shown in each bucket: 1440/Size entries.
	Days [][]string
	// Size is the bucket length in minutes; it divides 60.
	Size int
	// Mixed counts buckets that contain more than one state; they show the majority state, so the preview is
	// approximate.
	Mixed int
}

// segment is a stretch of equal buckets in one day, in minutes [from, to).
type segment struct {
	state string
	from  int
	to    int
}

// Downsample buckets the week. A bucket shows the state covering most of its minutes; ties go to the state that comes
// later in the configured state list.
func Downsample(out *schedule.Output, cfg *config.Config, size int) Preview {
	p := Preview{Size: size}

	for d := range days {
		minuteStates := make([]string, 0, minutesPerDay)
		for _, r := range dayRuns(out, d) {
			for range endMinutes(r.End) - minutes(r.Start) {
				minuteStates = append(minuteStates, r.State)
			}
		}

		buckets := make([]string, 0, minutesPerDay/size)
		for start := 0; start < minutesPerDay; start += size {
			state, mixed := majority(minuteStates[start:start+size], cfg)
			buckets = append(buckets, state)

			if mixed {
				p.Mixed++
			}
		}

		p.Days = append(p.Days, buckets)
	}

	return p
}

func majority(states []string, cfg *config.Config) (string, bool) {
	counts := map[string]int{}
	for _, t := range states {
		counts[t]++
	}

	best, bestCount, bestIndex := "", -1, -1

	for t, n := range counts {
		i, _ := stateInfo(cfg, t)
		if n > bestCount || (n == bestCount && i > bestIndex) {
			best, bestCount, bestIndex = t, n, i
		}
	}

	return best, len(counts) > 1
}

// segments merges equal neighbouring buckets of day d.
func (p Preview) segments(d int) []segment {
	var out []segment

	for i, state := range p.Days[d] {
		from := i * p.Size
		if n := len(out); n > 0 && out[n-1].state == state {
			out[n-1].to = from + p.Size

			continue
		}

		out = append(out, segment{state: state, from: from, to: from + p.Size})
	}

	return out
}
