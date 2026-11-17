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
	"maket/internal/timeline"
)

type speechMark struct {
	Start     int          `json:"start"`
	End       int          `json:"end"`
	StartTime float64      `json:"start_time"`
	EndTime   float64      `json:"end_time"`
	Value     string       `json:"value"`
	Chunks    []speechMark `json:"chunks"`
}

func speechifyDictionary(root string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(root, "voices", "stress.json"))
	if err != nil {
		return nil, err
	}
	var dictionary map[string]string
	if err := json.Unmarshal(b, &dictionary); err != nil {
		return nil, fmt.Errorf("voices/stress.json: %w", err)
	}
	return dictionary, nil
}

func speechifyTexts(root string, sc *scene.Scene) ([]string, error) {
	dictionary, err := speechifyDictionary(root)
	if err != nil {
		return nil, err
	}
	var texts []string
	for _, p := range sc.Phrases {
		texts = append(texts, restress(p.Text, dictionary, p.Stress))
	}
	return texts, nil
}

func synthesizeSpeechify(ctx context.Context, root string, cfg Config, sc *scene.Scene) (*Result, error) {
	if cfg.SpeechifyPython == "" || cfg.SpeechifyKeyFile == "" {
		return nil, fmt.Errorf("speechify_python and speechify_key_file are required; run bash tools/install.sh --speechify")
	}
	if cfg.SpeechifyVoice == "" {
		cfg.SpeechifyVoice = "alexei"
	}
	dictionary, err := speechifyDictionary(root)
	if err != nil {
		return nil, err
	}
	var texts []string
	overrides := [][2]string{}
	for _, p := range sc.Phrases {
		texts = append(texts, strings.ReplaceAll(p.Text, "\u0301", ""))
		for _, w := range scene.Words(p.Text) {
			v, ok := stressOf(dictionary, scene.Fold(w))
			if own, found := stressOf(p.Stress, scene.Fold(w)); found {
				v, ok = own, true
			}
			if ok {
				overrides = append(overrides, [2]string{w, v})
			}
		}
	}
	script, err := os.ReadFile(filepath.Join(root, "tools", "voice", "speechify.py"))
	if err != nil {
		return nil, err
	}
	keyData, _ := json.Marshal([]any{"speechify-v1", cfg.SpeechifyVoice, texts, overrides, script})
	key := sha256.Sum256(keyData)
	r := &run{ctx: ctx, root: root, cfg: cfg, sc: sc, dir: filepath.Join(root, ".cache", "voice", "speechify", hex.EncodeToString(key[:])[:24])}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, err
	}
	job := map[string]any{"voice": cfg.SpeechifyVoice, "texts": texts, "overrides": overrides, "key_file": cfg.SpeechifyKeyFile, "directory": r.dir}
	if err := r.python(cfg.SpeechifyPython, "speechify.py", job); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(r.dir, "response.json"))
	if err != nil {
		return nil, err
	}
	var response struct {
		Marks    speechMark `json:"speech_marks"`
		Billable int        `json:"billable_characters_count"`
	}
	if err := json.Unmarshal(b, &response); err != nil {
		return nil, fmt.Errorf("speechify response: %w", err)
	}
	var marks []speechMark
	var visit func(speechMark)
	visit = func(m speechMark) {
		if len(m.Chunks) == 0 {
			if strings.TrimSpace(m.Value) != "" {
				marks = append(marks, m)
			}
			return
		}
		for _, c := range m.Chunks {
			visit(c)
		}
	}
	visit(response.Marks)
	pcm, err := audio.Read(filepath.Join(r.dir, "source.wav"))
	if err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return nil, fmt.Errorf("speechify: empty audio")
	}
	res := &Result{Track: filepath.Join(r.dir, "voice.wav"), Warnings: []string{"speechify: scene voice / -voice and seed are ignored; using speechify_voice=" + cfg.SpeechifyVoice}}
	if len(marks) == 0 {
		return nil, fmt.Errorf("speechify: response has no speech marks")
	}
	previous := 0.0
	inputLength := utf8.RuneCountInString(strings.Join(texts, " "))
	for _, m := range marks {
		if m.Start < 0 || m.End <= m.Start || m.End > inputLength || m.StartTime < previous || m.EndTime <= m.StartTime || m.EndTime/1000 > float64(len(pcm))/audio.Rate+0.05 {
			return nil, fmt.Errorf("speechify: invalid speech mark for %q", m.Value)
		}
		previous = m.StartTime
	}
	var pieces []*piece
	used := make([]int, len(marks))
	offset := 0
	for i, text := range texts {
		pt := timeline.PhraseTime{}
		cursor := 0
		for _, word := range scene.Words(text) {
			startByte := cursor + strings.Index(text[cursor:], word)
			start := offset + utf8.RuneCountInString(text[:startByte])
			end := start + utf8.RuneCountInString(word)
			cursor = startByte + len(word)
			wt := timeline.WordTime{Text: word, S: math.Inf(1)}
			nearest, distance := 0, inputLength+1
			for j, m := range marks {
				if d := max(0, max(start-m.End, m.Start-end)); d < distance {
					nearest, distance = j, d
				}
				if m.End > start && m.Start < end {
					wt.S = math.Min(wt.S, timeline.Lead+m.StartTime/1000)
					wt.E = math.Max(wt.E, timeline.Lead+m.EndTime/1000)
					used[j]++
				}
			}
			if math.IsInf(wt.S, 1) {
				m := marks[nearest]
				wt.S, wt.E = timeline.Lead+m.StartTime/1000, timeline.Lead+m.EndTime/1000
				res.Warnings = append(res.Warnings, fmt.Sprintf("phrase %d: no speech mark for %q; using neighbouring mark", i+1, word))
			}
			pt.Words = append(pt.Words, wt)
		}
		if len(pt.Words) == 0 {
			return nil, fmt.Errorf("speechify: phrase %d has no words", i+1)
		}
		pt.Start, pt.End = pt.Words[0].S, pt.Words[len(pt.Words)-1].E
		p := &piece{pcm: pcm, from: pt.Start - timeline.Lead, to: pt.End - timeline.Lead}
		pieces = append(pieces, p)
		res.Tempo = append(res.Tempo, fmt.Sprintf("phrase %d: speechify %s, %.2f syl/s active, %.2f syl/s including pauses, no processing", i+1, cfg.SpeechifyVoice, r.speechRate(i, p), float64(vowels(text))/(pt.End-pt.Start)))
		if sc.Phrases[i].Pause != 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("phrase %d: explicit pause ignored by continuous speechify take", i+1))
		}

		res.Phrases = append(res.Phrases, pt)
		offset += utf8.RuneCountInString(text) + 1
	}
	for _, count := range used {
		if count > 1 {
			res.Warnings = append(res.Warnings, "speechify: a speech mark spans multiple words; their events share its timing")
			break
		}
	}
	whole := &piece{pcm: pcm, from: 0, to: float64(len(pcm)) / audio.Rate}
	if edgeQuiet(whole, true) < 0.1 {
		res.Warnings = append(res.Warnings, "speechify: less than 100 ms quiet at original audio end; check word ending")
	}
	if edgeQuiet(whole, false) < 0.02 {
		res.Warnings = append(res.Warnings, "speechify: less than 20 ms quiet at original audio start; check first word")
	}
	res.Tempo = append(res.Tempo, fmt.Sprintf("take: speechify %s, short pauses %.2f/s, tempo spread %.1f%%, billable characters %d", cfg.SpeechifyVoice, choppy(pieces), r.spread(pieces)*100, response.Billable))

	if choppy(pieces) > choppyMax {
		res.Warnings = append(res.Warnings, "speechify: too many short pauses; audio kept unchanged")
	}
	if cfg.TTSPython != "" && cfg.WhisperModel != "" {
		got, err := r.asrAll([]string{filepath.Join(r.dir, "source.wav")})
		if err != nil {
			return nil, err
		}
		var words []sceneWord
		spell := map[string]string{}
		for _, ov := range overrides {
			if !strings.HasPrefix(ov[1], "ipa:") {
				spell[scene.Fold(ov[0])] = scene.Fold(ov[1])
			}
		}
		for i, p := range sc.Phrases {
			for _, w := range scene.Words(p.Text) {
				words = append(words, sceneWord{text: w, phrase: i})
			}
		}
		owner := align(words, got[0], spell)
		for _, w := range words {
			if w.match < 0 || w.guess {
				res.Warnings = append(res.Warnings, fmt.Sprintf("phrase %d: not heard: %s", w.phrase+1, w.text))
			} else if norm(w.text, spell) != norm(got[0][w.match].W, spell) {
				res.Warnings = append(res.Warnings, fmt.Sprintf("phrase %d: recognition differs: %s / %s", w.phrase+1, w.text, got[0][w.match].W))
			}
		}
		for j, own := range owner {
			if own < 0 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("recognition extra word: %s at %.2f", got[0][j].W, got[0][j].S))
			}
		}
	} else {
		res.Warnings = append(res.Warnings, "speechify: recognition checks off; configure tts_python and whisper_model")
	}
	track := append(make([]float32, int(timeline.Lead*audio.Rate)), pcm...)
	res.Phrases[len(res.Phrases)-1].End = math.Max(res.Phrases[len(res.Phrases)-1].End, float64(len(track))/audio.Rate)
	if err := audio.Write(res.Track, track); err != nil {
		return nil, err
	}
	return res, nil
}
