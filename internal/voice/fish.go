package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"maket/internal/audio"
	"maket/internal/scene"
)

type fishServiceError struct{ error }

func (e fishServiceError) Unwrap() error { return e.error }

var fishRequestFailure = regexp.MustCompile(`^fish: phrase [1-9][0-9]*: (HTTP [0-9]{3}|connection failed)$`)

func fishWorkerError(detail string, err error, keyFile string) error {
	failure := fmt.Errorf("%s: %w", detail, err)
	message, _, _ := strings.Cut(detail, ";")
	switch message {
	case "fish: key file must contain one nonempty API key",
		"fish: synthesis failed (IncompleteRead)",
		"fish: synthesis failed (RemoteDisconnected)",
		"fish: synthesis failed (ConnectionResetError)",
		"fish: synthesis failed (ConnectionAbortedError)":
		return fishServiceError{failure}
	case "fish: synthesis failed (FileNotFoundError)":
		if _, statErr := os.Stat(keyFile); os.IsNotExist(statErr) {
			return fishServiceError{failure}
		}
	default:
		if fishRequestFailure.MatchString(message) {
			return fishServiceError{failure}
		}
	}
	return failure
}

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
	if spec.ReferenceID == "" && (spec.RefAudio == "" || strings.TrimSpace(spec.RefText) == "") {
		return nil, fmt.Errorf("fish: voice %q requires reference_id or ref_audio and ref_text", name)
	}
	cfg.Voice = name
	if cfg.FishModel == "" {
		cfg.FishModel = "s2.1-pro-free"
	}
	return &run{ctx: ctx, root: root, cfg: cfg, sc: sc, spec: spec, ref: filepath.Join(vdir, spec.RefAudio)}, nil
}

func prepareDialogue(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string, markOnly bool) (*dialogue, error) {
	if name == "" {
		name = cfg.Voice
	}
	names, err := phraseVoices(sc, name)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("voice: scene has no phrases")
	}
	for i, p := range sc.Phrases {
		if len(scene.Words(p.Text)) == 0 {
			return nil, fmt.Errorf("phrase %d has no words", i+1)
		}
	}
	if !markOnly && (cfg.TTSPython == "" || cfg.WhisperModel == "") {
		return nil, fmt.Errorf("fish: word timing requires tts_python and whisper_model in maket.local.json")
	}
	runs := make([]*run, len(names))
	loaded := map[string]*run{}
	for i, name := range names {
		if loaded[name] == nil {
			loaded[name], err = newFishRun(ctx, root, cfg, sc, name)
			if err != nil {
				return nil, err
			}
		}
		runs[i] = loaded[name]
	}
	texts := make([]string, len(runs))
	plain := strings.NewReplacer("+", "", "\u0301", "")
	for i, r := range runs {
		texts[i] = plain.Replace(restress(plain.Replace(sc.Phrases[i].Text), r.spec.Spell))
	}
	take := &dialogue{runs: runs, texts: texts}
	if !markOnly || cfg.Fallback != nil {
		if err := take.recordFish(); err != nil {
			var unavailable fishServiceError
			if errors.As(err, &unavailable) {
				return fallbackDialogue(ctx, root, cfg, sc, names, markOnly, err)
			}
			return nil, err
		}
	}
	return take, nil
}

func (d *dialogue) recordFish() error {
	r := d.runs[0]
	cfg := r.cfg
	if cfg.FishPython == "" {
		return fmt.Errorf("fish_python is required; run bash tools/install.sh --fish")
	}
	var items []map[string]any
	var identities []any
	for i, phraseRun := range d.runs {
		item := map[string]any{"text": d.texts[i]}
		if phraseRun.spec.ReferenceID != "" {
			item["reference_id"] = phraseRun.spec.ReferenceID
			identities = append(identities, []any{phraseRun.cfg.Voice, phraseRun.spec.ReferenceID})
		} else {
			ref, err := os.ReadFile(phraseRun.ref)
			if err != nil {
				return fmt.Errorf("voice %q sample: %w", phraseRun.cfg.Voice, err)
			}
			item["ref_audio"], item["ref_text"] = phraseRun.ref, phraseRun.spec.RefText
			identities = append(identities, []any{phraseRun.cfg.Voice, sha256.Sum256(ref), sha256.Sum256([]byte(phraseRun.spec.RefText))})
		}
		items = append(items, item)
	}
	script, err := os.ReadFile(filepath.Join(r.root, "tools", "voice", "fish.py"))
	if err != nil {
		return err
	}
	keyData, _ := json.Marshal([]any{"fish-v2", r.cfg.FishModel, identities, audio.Rate, d.texts, script})
	key := sha256.Sum256(keyData)
	r.dir = filepath.Join(r.root, ".cache", "voice", "fish", hex.EncodeToString(key[:])[:24])
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return err
	}
	if cfg.FishKeyFile == "" {
		return fishServiceError{errors.New("fish: fish_key_file is required")}
	}
	job := map[string]any{"model": r.cfg.FishModel, "items": items, "rate": audio.Rate, "key_file": cfg.FishKeyFile, "directory": r.dir}
	if err := r.python(cfg.FishPython, "fish.py", job); err != nil {
		return err
	}
	wavs := make([]string, len(d.texts))
	for i := range wavs {
		wavs[i] = filepath.Join(r.dir, fmt.Sprintf("phrase-%d.wav", i+1))
	}
	d.wavs, d.dir = wavs, r.dir
	return nil
}
