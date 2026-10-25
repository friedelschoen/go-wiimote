package commonhid

import (
	"iter"
	"slices"
)

// b=button, a=accel, i=ir, e=expansion
var reportingModes = map[byte]string{
	0x30: "bb",
	0x31: "bbaaa",
	0x32: "bbeeeeeeee",
	0x33: "bbaaaiiiiiiiiiiii",
	0x34: "bbeeeeeeeeeeeeeeeeeee",
	0x35: "bbaaaeeeeeeeeeeeeeeee",
	0x36: "bbiiiiiiiiiieeeeeeeee",
	0x37: "bbaaaiiiiiiiiiieeeeee",
	0x3d: "eeeeeeeeeeeeeeeeeeeee",
	0x3e: "bbaaaiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii", // interleaved, only selected if requested (i-max > 12)
}

var reportingPriority = []byte{0x32, 0x37, 0x35, 0x36, 0x34, 0x33, 0x31, 0x30, 0x3d, 0x3e}

func runecount(s string, b rune) (cnt int) {
	for _, chr := range s {
		if chr == b {
			cnt++
		}
	}
	return
}

func modeMatches(features string, req modeRequirement) bool {
	cnt := runecount(features, req.feature)
	return cnt >= req.min && cnt <= req.max
}

func findMode(openIfs iter.Seq[feature]) byte {
	// score less -> better
	type scoredMode struct {
		mode  byte
		score int
	}
	var modes []scoredMode

	for mode, features := range reportingModes {
		score := 0
		ok := true

		for p := range openIfs {
			reqs := p.requireModes()
			matched := false

			for i, req := range reqs {
				if modeMatches(features, req) {
					score += i
					matched = true
					break
				}
			}

			if !matched {
				ok = false
				break
			}
		}

		if ok {
			modes = append(modes, scoredMode{mode, score})
		}
	}
	if len(modes) == 0 {
		return 0
	}

	best := slices.MinFunc(modes, func(left, right scoredMode) int {
		if left.score != right.score {
			return right.score - left.score
		}
		return slices.Index(reportingPriority, right.mode) - slices.Index(reportingPriority, left.mode)
	})
	return best.mode
}
