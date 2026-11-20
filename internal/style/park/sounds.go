package park

import (
	"sort"

	"maket/internal/audio"
)

func (sh *shot) cue(t float64, kind string, dur float64) {
	sh.cues = append(sh.cues, audio.Cue{T: t, Kind: kind, Gain: 1, Dur: dur, Seed: sh.rng.Int63()})
}

func (sh *shot) soundtrack() {
	bed := audio.Cue{T: sh.at, Kind: audio.Calliope, Gain: 1, Dur: sh.end - sh.at}
	for _, o := range sh.objects {
		for _, span := range o.running {
			bed.Accents = append(bed.Accents, [2]float64{span.From - sh.at, span.To - sh.at})
			sh.tl.AddActivity(span.From, span.To)
		}
	}
	sh.cues = append(sh.cues, bed)
	sort.SliceStable(sh.cues, func(i, j int) bool { return sh.cues[i].T < sh.cues[j].T })
}
