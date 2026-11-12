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

	"maket/internal/audio"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style"
	"maket/internal/style/maket"
	"maket/internal/style/pixel"
	"maket/internal/timeline"
	"maket/internal/video"
	"maket/internal/voice"
)

var styles = map[string]func(root, look string, fonts *render.Fonts) (style.Style, error){
	"maket": func(root, _ string, fonts *render.Fonts) (style.Style, error) { return maket.New(root, fonts) },
	"none":  func(_, _ string, fonts *render.Fonts) (style.Style, error) { return style.NewNone(fonts), nil },
	"pixel": func(root, look string, fonts *render.Fonts) (style.Style, error) { return pixel.New(root, look, fonts) },
}

const usage = `maket render scene.json -o out.mp4 [-voice NAME] [-seed N] [-silent] [-workers N] [-root DIR]
maket stills scene.json -o dir [-voice NAME] [-silent] [-root DIR] t1 t2 ...
maket voice scene.json [-voice NAME] [-seed N] [-root DIR]
maket char list [-look L] [-root DIR]
maket char grid NAME [STATE] [-i N] [-look L] [-root DIR]
maket char build GRID.txt -o DRAFT
maket char check DRAFT|NAME [-look L] [-root DIR]
maket char sheet DRAFT|NAME -o sheet.png [-ref a,b] [-look L] [-root DIR]
maket char add DRAFT -name N -label L [-alias a,b] [-look L] [-root DIR]`

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "char" {
		os.Exit(charMain(os.Args[2], os.Args[3:]))
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, scenePath := os.Args[1], os.Args[2]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	out := fs.String("o", "", "output file or directory")
	workers := fs.Int("workers", runtime.NumCPU(), "frames rendered in parallel")
	root := fs.String("root", ".", "repository root with assets/")
	voiceName := fs.String("voice", "", "voice folder in voices/, default from maket.json or the scene")
	seed := fs.Int("seed", 0, "voice seed, default from the scene")
	silent := fs.Bool("silent", false, "no voice: phrases timed by letter count")
	fs.Parse(os.Args[3:])
	opt := options{root: *root, voice: *voiceName, seed: *seed, silent: *silent}
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "maket: -workers must be at least 1")
		os.Exit(2)
	}

	var err error
	switch cmd {
	case "render":
		err = renderVideo(scenePath, *out, opt, *workers)
	case "stills":
		err = stills(scenePath, *out, opt, fs.Args())
	case "voice":
		err = voiceTable(scenePath, opt)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "maket:", err)
		os.Exit(1)
	}
}

type options struct {
	root, voice string
	seed        int
	silent      bool
}

type job struct {
	sc    *scene.Scene
	tl    *timeline.Timeline
	st    style.Style
	voice *voice.Result
}

func speak(sc *scene.Scene, opt options) (*voice.Result, error) {
	cfg, err := voice.LoadConfig(opt.root)
	if err != nil {
		return nil, err
	}
	name := opt.voice
	if name == "" {
		name = sc.Voice
	}
	if opt.seed != 0 {
		sc.Seed = opt.seed
	}
	res, err := voice.Synthesize(context.Background(), opt.root, cfg, sc, name)
	if err != nil {
		return nil, err
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "maket:", w)
	}
	return res, nil
}

func prepare(scenePath string, opt options) (*job, error) {
	root := opt.root
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
	if sc.Look != "" && name != "pixel" {
		return nil, fmt.Errorf("style %q has no looks, look %q needs style pixel", name, sc.Look)
	}
	if len(sc.Shots) > 0 && name != "pixel" {
		return nil, fmt.Errorf("style %q has no shots, shots need style pixel", name)
	}
	st, err := mk(root, sc.Look, fonts)
	if err != nil {
		return nil, err
	}
	tl := timeline.Estimate(sc)
	var said *voice.Result
	if !opt.silent {
		if said, err = speak(sc, opt); err != nil {
			return nil, err
		}
		tl = timeline.New(sc, said.Phrases)
	}
	if _, err := tl.Moments(sc); err != nil {
		return nil, err
	}
	if err := st.Layout(sc, tl); err != nil {
		return nil, err
	}
	for _, g := range tl.Quiet(sc) {
		fmt.Fprintf(os.Stderr, "maket: nothing happens from %.1f to %.1f s\n", g.From, g.To)
	}
	return &job{sc: sc, tl: tl, st: st, voice: said}, nil
}

func (j *job) frame(t float64) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, render.W, render.H))
	j.st.Draw(im, t)
	render.Overlay(im, j.sc, j.tl, j.st.Theme(), t)
	return im
}

func renderVideo(scenePath, out string, opt options, workers int) error {
	if out == "" {
		return fmt.Errorf("render: -o is required")
	}
	j, err := prepare(scenePath, opt)
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
	track := ""
	if j.voice != nil {
		track = j.voice.Track
	}
	mix := out + ".audio.wav"
	defer os.Remove(mix)
	if err := audio.Mix(track, j.st.Sounds(), j.tl.Duration, mix); err != nil {
		return err
	}
	if err := video.Mux(vid, mix, out); err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func voiceTable(scenePath string, opt options) error {
	sc, err := scene.Load(scenePath)
	if err != nil {
		return err
	}
	res, err := speak(sc, opt)
	if err != nil {
		return err
	}
	for i, p := range res.Phrases {
		fmt.Printf("%2d  %6.2f  %6.2f  %s\n", i+1, p.Start, p.End, sc.Phrases[i].Text)
	}
	fmt.Println(res.Track)
	return nil
}

func stills(scenePath, dir string, opt options, times []string) error {
	if dir == "" {
		return fmt.Errorf("stills: -o is required")
	}
	j, err := prepare(scenePath, opt)
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
