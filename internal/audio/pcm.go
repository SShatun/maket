package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
)

const Rate = 48000

func Read(path string, filters ...string) ([]float32, error) {
	args := []string{"-v", "error", "-i", path}
	if len(filters) > 0 {
		args = append(args, "-af", filters[0])
	}
	args = append(args, "-f", "f32le", "-ac", "1", "-ar", fmt.Sprint(Rate), "-")
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg read %s: %w", path, err)
	}
	pcm := make([]float32, len(out)/4)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(out[i*4:]))
	}
	return pcm, nil
}

func Write(path string, pcm []float32) error {
	var b bytes.Buffer
	b.Grow(len(pcm) * 4)
	for _, v := range pcm {
		binary.Write(&b, binary.LittleEndian, v)
	}
	cmd := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "f32le", "-ac", "1", "-ar", fmt.Sprint(Rate), "-i", "-", "-c:a", "pcm_s16le", path)
	cmd.Stdin = &b
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg write %s: %w", path, err)
	}
	return nil
}
