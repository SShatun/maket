package video

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

func Encode(ctx context.Context, out string, w, h, fps, n int, frame func(i int) *image.RGBA, workers int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", "rgb24", "-s", fmt.Sprintf("%dx%d", w, h), "-r", strconv.Itoa(fps), "-i", "-",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p", "-movflags", "+faststart", out)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	results := make([]chan []byte, n)
	for i := range results {
		results[i] = make(chan []byte, 1)
	}
	window := make(chan struct{}, workers*2)
	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for i := 0; i < n; i++ {
			select {
			case window <- struct{}{}:
			case <-ctx.Done():
				return
			}
			jobs <- i
		}
	}()
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] <- rgb24(frame(i))
			}
		}()
	}

	bw := bufio.NewWriterSize(stdin, 1<<20)
	var werr error
	for i := 0; i < n && werr == nil; i++ {
		_, werr = bw.Write(<-results[i])
		<-window
	}
	if werr == nil {
		werr = bw.Flush()
	}
	stdin.Close()
	if werr != nil {
		cancel()
	}
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return werr
}

func rgb24(im *image.RGBA) []byte {
	b := im.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := im.Pix[im.PixOffset(b.Min.X, y):im.PixOffset(b.Max.X, y)]
		for x := 0; x < len(row); x += 4 {
			out = append(out, row[x], row[x+1], row[x+2])
		}
	}
	return out
}

func Mux(videoPath, audioPath, out string) error {
	tmp := out + ".part.mp4"
	args := []string{"-v", "error", "-y", "-i", videoPath}
	if audioPath != "" {
		args = append(args, "-i", audioPath, "-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "aac", "-b:a", "160k", "-shortest", tmp)
	} else {
		args = append(args, "-c", "copy", tmp)
	}
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mux: %w", err)
	}
	return os.Rename(tmp, out)
}
