package voice

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"unicode/utf8"

	"maket/internal/audio"
	"maket/internal/scene"
)

type dialogue struct {
	runs  []*run
	texts []string
	wavs  []string
	dir   string
}

func fallbackDialogue(ctx context.Context, root string, cfg Config, sc *scene.Scene, names []string, markOnly bool, fishErr error) (*dialogue, error) {
	if cfg.Fallback == nil || ctx.Err() != nil {
		return nil, fishErr
	}
	if cfg.Fallback.Backend != "qwen" {
		return nil, fmt.Errorf("%w; unsupported local fallback backend %q", fishErr, cfg.Fallback.Backend)
	}
	fmt.Fprintf(os.Stderr, "maket: %v; voiced locally with the fallback voices\n", fishErr)
	cfg.Backend = cfg.Fallback.Backend
	take, err := prepareLocalDialogue(ctx, root, cfg, sc, names, markOnly)
	if err != nil {
		return nil, fmt.Errorf("%s fallback: %w", cfg.Backend, err)
	}
	return take, nil
}

func prepareLocalDialogue(ctx context.Context, root string, cfg Config, sc *scene.Scene, names []string, markOnly bool) (*dialogue, error) {
	groups := map[string][]int{}
	for i, name := range names {
		if clone := cfg.Fallback.Voices[name]; clone != "" {
			name = clone
		}
		groups[name] = append(groups[name], i)
	}
	d := &dialogue{runs: make([]*run, len(names)), texts: make([]string, len(names)), wavs: make([]string, len(names))}
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		indices := groups[name]
		part := *sc
		part.Phrases = make([]scene.Phrase, len(indices))
		for j, i := range indices {
			part.Phrases[j] = sc.Phrases[i]
		}
		cfg.Voice = name
		r, spec, err := newRun(ctx, root, cfg, &part, name)
		if err != nil {
			return nil, fmt.Errorf("voice %q: %w", name, err)
		}
		marked, err := r.stress()
		if err != nil {
			return nil, err
		}
		for j, i := range indices {
			d.texts[i] = marked[j]
		}
		if markOnly {
			continue
		}
		key, err := r.key(spec, marked)
		if err != nil {
			return nil, err
		}
		r.dir = filepath.Join(r.root, ".cache", "voice", "qwen", key)
		if err := os.MkdirAll(r.dir, 0o755); err != nil {
			return nil, err
		}
		wavs, err := r.recordLocalPhrases(marked)
		if err != nil {
			return nil, err
		}
		for j, i := range indices {
			d.runs[i], d.wavs[i] = r, wavs[j]
		}
	}
	if markOnly {
		return d, nil
	}
	data, err := json.Marshal([]any{d.wavs, sc.Phrases})
	if err != nil {
		return nil, err
	}
	d.dir = filepath.Join(d.runs[0].root, ".cache", "voice", "qwen", fmt.Sprintf("scene-%.12x", sha256.Sum256(data)))
	return d, os.MkdirAll(d.dir, 0o755)
}

func (r *run) recordLocalPhrases(marked []string) ([]string, error) {
	wavs := make([]string, len(marked))
	misses := make([]int, len(marked))
	for attempt := range takes {
		seed := r.sc.Seed + attempt
		var items []ttsItem
		var paths []string
		var indices []int
		for i, text := range marked {
			if attempt > 0 && misses[i] == 0 {
				continue
			}
			wav := filepath.Join(r.dir, fmt.Sprintf("phrase-%d-%d-%.6x.wav", i+1, seed, sha256.Sum256([]byte(text))))
			items = append(items, ttsItem{Text: text, Out: wav})
			paths = append(paths, wav)
			indices = append(indices, i)
		}
		if len(items) == 0 {
			break
		}
		if err := r.tts(items, seed); err != nil {
			return nil, err
		}
		got, err := r.asrAll(paths)
		if err != nil {
			return nil, err
		}
		for j, i := range indices {
			missed := recognitionErrors(r.sceneWords(i), got[j], r.spec.Spell)
			if attempt == 0 || missed < misses[i] {
				wavs[i], misses[i] = paths[j], missed
			}
		}
	}
	return wavs, nil
}

func recognitionErrors(words []sceneWord, got []heard, spell map[string]string) int {
	owner := align(words, got, spell)
	numbers := heardNumbers(got)
	missed := 0
	for _, w := range words {
		if w.match < 0 || w.guess || (numbers[w.match].count == 0 && norm(w.text, spell) != norm(got[w.match].W, spell)) {
			missed++
		}
	}
	for _, phrase := range owner {
		if phrase < 0 {
			missed++
		}
	}
	return missed
}

func (d *dialogue) place(sc *scene.Scene) (*Result, error) {
	r := *d.runs[0]
	r.sc, r.dir = sc, d.dir
	backend := r.cfg.Backend
	if backend == "fish" {
		backend += " " + r.cfg.FishModel
	}
	got, err := r.asrAll(d.wavs)
	if err != nil {
		return nil, err
	}
	var warnings []string
	if r.cfg.Backend == "fish" {
		warnings = append(warnings, "fish: seed is ignored; Russian stress control is undocumented, sending plain text with voice spell replacements")
	}
	var pieces []*piece
	var tempo []string
	characters := 0
	for i, phrase := range sc.Phrases {
		spell := d.runs[i].spec.Spell
		pcm, err := audio.Read(d.wavs[i])
		if err != nil {
			return nil, err
		}
		if len(pcm) == 0 {
			return nil, fmt.Errorf("%s: phrase %d has empty audio", r.cfg.Backend, i+1)
		}
		duration := float64(len(pcm)) / audio.Rate
		words := r.sceneWords(i)
		owner := align(words, got[i], spell)
		for _, w := range words {
			if w.match >= 0 && !w.guess && norm(w.text, spell) != norm(got[i][w.match].W, spell) {
				warnings = append(warnings, fmt.Sprintf("phrase %d: recognition differs: %s / %s", i+1, w.text, got[i][w.match].W))
			}
		}
		for j, own := range owner {
			if own < 0 {
				warnings = append(warnings, fmt.Sprintf("phrase %d: recognition extra word: %s at %.2f", i+1, got[i][j].W, got[i][j].S))
			}
		}
		end := duration
		if r.cfg.Backend == "qwen" && len(got[i]) > 0 {
			end = got[i][len(got[i])-1].E
		}
		words = append(words, sceneWord{match: 0, s: end, e: end})
		fillTimes(words)
		words = words[:len(words)-1]
		for j := range words {
			words[j].s = math.Max(0, math.Min(duration, words[j].s))
			words[j].e = math.Max(words[j].s, math.Min(duration, words[j].e))
		}
		p := &piece{pcm: pcm, to: duration, words: words}
		pieces = append(pieces, p)
		tempo = append(tempo, fmt.Sprintf("phrase %d: %s %s, %.2f syl/s active, %.2f syl/s including pauses, no tempo processing", i+1, backend, d.runs[i].cfg.Voice, r.speechRate(i, p), float64(vowels(phrase.Text))/duration))
		characters += utf8.RuneCountInString(d.texts[i])
	}
	res, err := r.place(pieces)
	if err != nil {
		return nil, err
	}
	if r.cfg.Backend == "fish" {
		last := len(res.Phrases) - 1
		res.Phrases[last].End = res.Phrases[last].Start + pieces[last].to
	}
	res.Warnings = append(warnings, res.Warnings...)
	summary := fmt.Sprintf("take: %s, short pauses %.2f/s, tempo spread %.1f%%", backend, choppy(pieces), r.spread(pieces)*100)
	if r.cfg.Backend == "fish" {
		summary += fmt.Sprintf(", characters sent %d", characters)
	}
	res.Tempo = append(tempo, summary)
	return res, nil
}
