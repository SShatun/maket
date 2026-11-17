package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"maket/internal/audio"
	"maket/internal/scene"
)

func newFishRun(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string) (*run, error) {
	if name == "" {
		name = cfg.Voice
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	vdir := filepath.Join(root, "voices", name)
	b, err := os.ReadFile(filepath.Join(vdir, "voice.json"))
	if err != nil {
		return nil, fmt.Errorf("voice %q: %w", name, err)
	}
	var spec Spec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("voice %q: %w", name, err)
	}
	if spec.RefAudio == "" || strings.TrimSpace(spec.RefText) == "" {
		return nil, fmt.Errorf("fish: voice %q requires ref_audio and ref_text", name)
	}
	cfg.Voice = name
	if cfg.FishModel == "" {
		cfg.FishModel = "s2.1-pro-free"
	}
	return &run{ctx: ctx, root: root, cfg: cfg, sc: sc, spec: spec, ref: filepath.Join(vdir, spec.RefAudio)}, nil
}

func fishTexts(r *run) []string {
	texts := make([]string, len(r.sc.Phrases))
	plain := strings.NewReplacer("+", "", "\u0301", "")
	for i, p := range r.sc.Phrases {
		texts[i] = plain.Replace(restress(plain.Replace(p.Text), r.spec.Spell))
	}
	return texts
}

func synthesizeFish(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string) (*Result, error) {
	if cfg.FishPython == "" || cfg.FishKeyFile == "" {
		return nil, fmt.Errorf("fish_python and fish_key_file are required; run bash tools/install.sh --fish")
	}
	if cfg.TTSPython == "" || cfg.WhisperModel == "" {
		return nil, fmt.Errorf("fish: word timing requires tts_python and whisper_model in maket.local.json")
	}
	r, err := newFishRun(ctx, root, cfg, sc, name)
	if err != nil {
		return nil, err
	}
	texts := fishTexts(r)
	ref, err := os.ReadFile(r.ref)
	if err != nil {
		return nil, fmt.Errorf("voice sample: %w", err)
	}
	script, err := os.ReadFile(filepath.Join(r.root, "tools", "voice", "fish.py"))
	if err != nil {
		return nil, err
	}
	keyData, _ := json.Marshal([]any{"fish-v1", r.cfg.FishModel, r.cfg.Voice, sha256.Sum256(ref), sha256.Sum256([]byte(r.spec.RefText)), audio.Rate, texts, script})
	key := sha256.Sum256(keyData)
	r.dir = filepath.Join(r.root, ".cache", "voice", "fish", hex.EncodeToString(key[:])[:24])
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, err
	}
	job := map[string]any{"model": r.cfg.FishModel, "ref_audio": r.ref, "ref_text": r.spec.RefText, "texts": texts, "rate": audio.Rate, "key_file": cfg.FishKeyFile, "directory": r.dir}
	if err := r.python(cfg.FishPython, "fish.py", job); err != nil {
		return nil, err
	}
	wavs := make([]string, len(texts))
	for i := range wavs {
		wavs[i] = filepath.Join(r.dir, fmt.Sprintf("phrase-%d.wav", i+1))
	}
	got, err := r.asrAll(wavs)
	if err != nil {
		return nil, err
	}
	warnings := []string{"fish: seed is ignored; Russian stress control is undocumented, sending plain text with voice spell replacements"}
	var pieces []*piece
	var tempo []string
	characters := 0
	for i, phrase := range sc.Phrases {
		pcm, err := audio.Read(wavs[i])
		if err != nil {
			return nil, err
		}
		if len(pcm) == 0 {
			return nil, fmt.Errorf("fish: phrase %d has empty audio", i+1)
		}
		duration := float64(len(pcm)) / audio.Rate
		var words []sceneWord
		for _, w := range scene.Words(phrase.Text) {
			words = append(words, sceneWord{text: w, phrase: i})
		}
		if len(words) == 0 {
			return nil, fmt.Errorf("fish: phrase %d has no words", i+1)
		}
		owner := align(words, got[i], r.spec.Spell)
		for _, w := range words {
			if w.match >= 0 && !w.guess && norm(w.text, r.spec.Spell) != norm(got[i][w.match].W, r.spec.Spell) {
				warnings = append(warnings, fmt.Sprintf("phrase %d: recognition differs: %s / %s", i+1, w.text, got[i][w.match].W))
			}
		}
		for j, own := range owner {
			if own < 0 {
				warnings = append(warnings, fmt.Sprintf("phrase %d: recognition extra word: %s at %.2f", i+1, got[i][j].W, got[i][j].S))
			}
		}
		words = append(words, sceneWord{match: 0, s: duration, e: duration})
		fillTimes(words)
		words = words[:len(words)-1]
		for j := range words {
			words[j].s = math.Max(0, math.Min(duration, words[j].s))
			words[j].e = math.Max(words[j].s, math.Min(duration, words[j].e))
		}
		p := &piece{pcm: pcm, to: duration, words: words}
		pieces = append(pieces, p)
		tempo = append(tempo, fmt.Sprintf("phrase %d: fish %s %s, %.2f syl/s active, %.2f syl/s including pauses, no tempo processing", i+1, r.cfg.FishModel, r.cfg.Voice, r.speechRate(i, p), float64(vowels(phrase.Text))/duration))
		characters += utf8.RuneCountInString(texts[i])
	}
	res, err := r.place(pieces)
	if err != nil {
		return nil, err
	}
	last := len(res.Phrases) - 1
	res.Phrases[last].End = res.Phrases[last].Start + pieces[last].to
	res.Warnings = append(warnings, res.Warnings...)
	res.Tempo = append(tempo, fmt.Sprintf("take: fish %s %s, short pauses %.2f/s, tempo spread %.1f%%, characters sent %d", r.cfg.FishModel, r.cfg.Voice, choppy(pieces), r.spread(pieces)*100, characters))
	return res, nil
}
