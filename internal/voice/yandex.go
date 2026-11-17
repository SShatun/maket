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
	"unicode/utf8"

	"maket/internal/audio"
	"maket/internal/scene"
)

func yandexTexts(root string, sc *scene.Scene) ([]string, error) {
	texts, err := speechifyTexts(root, sc)
	if err != nil {
		return nil, err
	}
	for i, text := range texts {
		texts[i] = plusForm(text)
	}
	return texts, nil
}

func synthesizeYandex(ctx context.Context, root string, cfg Config, sc *scene.Scene) (*Result, error) {
	if cfg.YandexPython == "" || cfg.YandexKeyFile == "" {
		return nil, fmt.Errorf("yandex_python and yandex_key_file are required; run bash tools/install.sh --yandex")
	}
	if cfg.TTSPython == "" || cfg.WhisperModel == "" {
		return nil, fmt.Errorf("yandex: word timing requires tts_python and whisper_model in maket.local.json")
	}
	if cfg.YandexVoice == "" {
		cfg.YandexVoice = "anton"
	}
	if cfg.YandexRole == "" {
		cfg.YandexRole = "good"
	}
	texts, err := yandexTexts(root, sc)
	if err != nil {
		return nil, err
	}
	dictionary, err := speechifyDictionary(root)
	if err != nil {
		return nil, err
	}
	script, err := os.ReadFile(filepath.Join(root, "tools", "voice", "yandex.py"))
	if err != nil {
		return nil, err
	}
	keyData, _ := json.Marshal([]any{"yandex-v1", cfg.YandexVoice, cfg.YandexRole, audio.Rate, texts, script})
	key := sha256.Sum256(keyData)
	r := &run{ctx: ctx, root: root, cfg: cfg, sc: sc, dir: filepath.Join(root, ".cache", "voice", "yandex", hex.EncodeToString(key[:])[:24])}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, err
	}
	job := map[string]any{"voice": cfg.YandexVoice, "role": cfg.YandexRole, "texts": texts, "rate": audio.Rate, "key_file": cfg.YandexKeyFile, "directory": r.dir}
	if err := r.python(cfg.YandexPython, "yandex.py", job); err != nil {
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
	warnings := []string{"yandex: scene voice / -voice and seed are ignored; using yandex_voice=" + cfg.YandexVoice + ", yandex_role=" + cfg.YandexRole}
	var pieces []*piece
	var tempo []string
	characters := 0
	for i, phrase := range sc.Phrases {
		pcm, err := audio.Read(wavs[i])
		if err != nil {
			return nil, err
		}
		if len(pcm) == 0 {
			return nil, fmt.Errorf("yandex: phrase %d has empty audio", i+1)
		}
		duration := float64(len(pcm)) / audio.Rate
		var words []sceneWord
		spell := map[string]string{}
		for _, w := range scene.Words(phrase.Text) {
			words = append(words, sceneWord{text: w, phrase: i})
			v, ok := stressOf(dictionary, scene.Fold(w))
			if own, found := stressOf(phrase.Stress, scene.Fold(w)); found {
				v, ok = own, true
			}
			if ok {
				spell[scene.Fold(w)] = scene.Fold(v)
			}
		}
		if len(words) == 0 {
			return nil, fmt.Errorf("yandex: phrase %d has no words", i+1)
		}
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
		words = append(words, sceneWord{match: 0, s: duration, e: duration})
		fillTimes(words)
		words = words[:len(words)-1]
		for j := range words {
			words[j].s = math.Max(0, math.Min(duration, words[j].s))
			words[j].e = math.Max(words[j].s, math.Min(duration, words[j].e))
		}
		p := &piece{pcm: pcm, to: duration, words: words}
		pieces = append(pieces, p)
		tempo = append(tempo, fmt.Sprintf("phrase %d: yandex %s %s, %.2f syl/s active, %.2f syl/s including pauses, no processing", i+1, cfg.YandexVoice, cfg.YandexRole, r.speechRate(i, p), float64(vowels(phrase.Text))/duration))
		characters += utf8.RuneCountInString(texts[i])
	}
	res, err := r.place(pieces)
	if err != nil {
		return nil, err
	}
	last := len(res.Phrases) - 1
	res.Phrases[last].End = res.Phrases[last].Start + pieces[last].to
	res.Warnings = append(warnings, res.Warnings...)
	res.Tempo = append(tempo, fmt.Sprintf("take: yandex %s %s, short pauses %.2f/s, tempo spread %.1f%%, characters sent %d", cfg.YandexVoice, cfg.YandexRole, choppy(pieces), r.spread(pieces)*100, characters))
	return res, nil
}
