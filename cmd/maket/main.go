package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style"
	"maket/internal/timeline"
	"maket/internal/video"
)

var styles = map[string]func(root string, fonts *render.Fonts) (style.Style, error){
	"none": func(_ string, fonts *render.Fonts) (style.Style, error) { return style.NewNone(fonts), nil },
}

const usage = `maket render scene.json -o out.mp4 [-workers N] [-root DIR]
maket stills scene.json -o dir [-root DIR] t1 t2 ...`

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, scenePath := os.Args[1], os.Args[2]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	out := fs.String("o", "", "output file or directory")
	workers := fs.Int("workers", runtime.NumCPU(), "frames rendered in parallel")
	root := fs.String("root", ".", "repository root with assets/")
	fs.Parse(os.Args[3:])
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "maket: -workers must be at least 1")
		os.Exit(2)
	}

	var err error
	switch cmd {
	case "render":
		err = renderVideo(scenePath, *out, *root, *workers)
	case "stills":
		err = stills(scenePath, *out, *root, fs.Args())
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "maket:", err)
		os.Exit(1)
	}
}

type job struct {
	sc *scene.Scene
	tl *timeline.Timeline
	st style.Style
}

func prepare(scenePath, root string) (*job, error) {
	sc, err := scene.Load(scenePath)
	if err != nil {
		return nil, err
	}
	fonts, err := render.LoadFonts(filepath.Join(root, "assets", "fonts"))
	if err != nil {
		return nil, err
	}
	name := sc.Style
	if name == "" {
		name = "none"
	}
	mk, ok := styles[name]
	if !ok {
		return nil, fmt.Errorf("unknown style %q", name)
	}
	st, err := mk(root, fonts)
	if err != nil {
		return nil, err
	}
	tl := timeline.Estimate(sc)
	if _, err := tl.Moments(sc); err != nil {
		return nil, err
	}
	if err := st.Layout(sc, tl); err != nil {
		return nil, err
	}
	for _, g := range tl.Quiet(sc) {
		fmt.Fprintf(os.Stderr, "maket: nothing happens from %.1f to %.1f s\n", g.From, g.To)
	}
	return &job{sc: sc, tl: tl, st: st}, nil
}

func (j *job) frame(t float64) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, render.W, render.H))
	j.st.Draw(im, t)
	render.Overlay(im, j.sc, j.tl, j.st.Theme(), t)
	return im
}

func renderVideo(scenePath, out, root string, workers int) error {
	if out == "" {
		return fmt.Errorf("render: -o is required")
	}
	j, err := prepare(scenePath, root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	n := int(math.Round(j.tl.Duration * render.FPS))
	vid := out + ".video.mp4"
	defer os.Remove(vid)
	err = video.Encode(context.Background(), vid, render.W, render.H, render.FPS, n, func(i int) *image.RGBA {
		return j.frame(float64(i) / render.FPS)
	}, workers)
	if err != nil {
		return err
	}
	if err := video.Mux(vid, "", out); err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func stills(scenePath, dir, root string, times []string) error {
	if dir == "" {
		return fmt.Errorf("stills: -o is required")
	}
	j, err := prepare(scenePath, root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, s := range times {
		t, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("stills: bad time %q", s)
		}
		path := filepath.Join(dir, fmt.Sprintf("still_%06.2f.png", t))
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		err = png.Encode(f, j.frame(t))
		f.Close()
		if err != nil {
			return err
		}
		fmt.Println(path)
	}
	return nil
}
