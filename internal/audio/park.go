package audio

import (
	"math"
	"math/rand"
)

func parkNote(dst []float64, start, frequency, duration, gain float64) {
	first := samples(start)
	end := min(len(dst), first+samples(duration))
	for i := first; i < end; i++ {
		t := float64(i-first) / Rate
		phase := 2 * math.Pi * frequency * t
		wave := math.Sin(phase) + 0.135*sgn(math.Sin(2*phase)) + 0.25*math.Sin(3*phase)
		env := math.Min(1, t/0.01) * math.Exp(-t/(0.6*duration))
		dst[i] += wave * (1 + 0.25*math.Sin(2*math.Pi*6*t)) * env * gain
	}
}

func calliope(dur float64, accents [][2]float64) []float64 {
	sig := make([]float64, samples(dur))
	melody := [...]float64{523, 659, 784, 659, 587, 698, 880, 698, 523, 659, 784, 1046}
	bass := [...]float64{131, 98}
	for k := 0; float64(k)*0.42 < dur; k++ {
		t := float64(k) * 0.42
		if k%3 == 0 {
			parkNote(sig, t, bass[k/3%2], 0.36, 0.35)
		} else {
			parkNote(sig, t, 392, 0.25, 0.18)
		}
		gain := 0.22
		for _, span := range accents {
			if t >= span[0] && t < span[1] {
				gain = .42
				break
			}
		}
		parkNote(sig, t, melody[k%len(melody)], 0.36, gain)
	}
	return sig
}

func parkCrowd(dur float64, rng *rand.Rand) []float64 {
	sig := make([]float64, samples(dur))
	filtered := 0.0
	for i := range sig {
		t := float64(i) / Rate
		filtered += 0.02 * (rng.NormFloat64() - filtered)
		sig[i] = filtered * 6 * (0.6 + 0.4*math.Sin(3*t)) * 0.5
	}
	return sig
}

func parkClick(rng *rand.Rand) []float64 {
	sig := make([]float64, samples(0.14))
	for i := range sig[:samples(0.08)] {
		t := float64(i) / Rate
		sig[i] = rng.NormFloat64() * math.Exp(-t/0.015) * 0.8 * 0.6
	}
	parkNote(sig, 0.02, 880, 0.12, 0.3)
	return sig
}

func parkChord() []float64 {
	sig := make([]float64, samples(1.44))
	for i, f := range [...]float64{523, 659, 784, 1046} {
		parkNote(sig, float64(i)*0.08, f, 1.2, 0.4)
	}
	return sig
}
