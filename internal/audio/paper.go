package audio

import (
	"math"
	"math/rand"
)

func strum(chord int, rng *rand.Rand) []float64 {
	chords := [4][]float64{
		{98, 146.83, 196, 246.94, 293.66, 392},
		{146.83, 220, 293.66, 369.99},
		{82.41, 123.47, 164.81, 196, 246.94, 329.63},
		{130.81, 164.81, 196, 261.63, 329.63},
	}
	var out []float64
	for i, hz := range chords[(chord%len(chords)+len(chords))%len(chords)] {
		ring := noise(max(2, int(Rate/hz)), rng)
		sig := make([]float64, samples(1.4))
		for j := range sig {
			p := j % len(ring)
			sig[j] = ring[p] * 0.25 * math.Min(1, float64(len(sig)-j)/(Rate*0.08))
			ring[p] = 0.496 * (ring[p] + ring[(p+1)%len(ring)])
		}
		out = at(out, float64(i)*0.018, sig)
	}
	return out
}

func babble(pitch, dur float64, rng *rand.Rand) []float64 {
	if pitch <= 0 {
		pitch = 180
	}
	out := make([]float64, max(0, samples(dur)))
	for start := 0; start < len(out); {
		n := samples(0.07 + rng.Float64()*0.07)
		hz := pitch * (0.85 + rng.Float64()*0.4)
		for j := 0; j < n && start+j < len(out); j++ {
			ph := 2 * math.Pi * hz * float64(j) / Rate
			fade := math.Sin(math.Pi * float64(j) / float64(n))
			out[start+j] = (math.Sin(ph) + 0.35*math.Sin(2*ph) + 0.18*math.Sin(3*ph)) * fade
		}
		start += n + samples(0.025+rng.Float64()*0.025)
	}
	return out
}

func chalk(dur float64, rng *rand.Rand) []float64 {
	out := make([]float64, max(0, samples(dur)))
	for start := 0; start < len(out); start += samples(0.09) {
		click := lowpass(noise(samples(0.03), rng), 0.15)
		for j, v := range click {
			if start+j >= len(out) {
				break
			}
			out[start+j] = v * math.Sin(math.Pi*float64(j)/float64(len(click)))
		}
	}
	return out
}
