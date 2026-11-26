package audio

import (
	"fmt"
	"math"
)

func addMusic(mix []float64, c Cue) error {
	track, err := Read(c.MusicFile)
	if err != nil {
		return err
	}
	if len(track) == 0 {
		return fmt.Errorf("empty music track: %s", c.MusicFile)
	}
	start := samples(c.T)
	length := min(samples(c.Dur), len(mix)-start)
	overlap := min(12*Rate, len(track)/2)
	stride := len(track) - overlap
	for i := max(0, -start); i < length; i++ {
		pos := i
		if length > len(track) && i >= stride {
			pos %= stride
		}
		sample := float64(track[pos])
		if pos != i && pos < overlap {
			t := float64(pos) / float64(overlap)
			sample = float64(track[stride+pos])*math.Cos(t*math.Pi/2) + sample*math.Sin(t*math.Pi/2)
		}
		fade := min(1., float64(min(i, length-1-i))/Rate)
		mix[start+i] += sample * c.Gain * fade
	}
	return nil
}

func duckMusic(music []float64, voice []float32) {
	const threshold = 0.036
	const ratio = 8.
	const attack = 4 / (0.020 * Rate)
	const release = 4 / (0.400 * Rate)
	var power float64
	for i := range music {
		var level float64
		if i < len(voice) {
			level = float64(voice[i]) * float64(voice[i])
		}
		coefficient := release
		if level > power {
			coefficient = attack
		}
		power += coefficient * (level - power)
		if power > threshold*threshold {
			music[i] *= math.Pow(threshold*threshold/power, (1-1/ratio)/2)
		}
	}
}
