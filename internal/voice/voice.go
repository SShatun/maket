package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"maket/internal/audio"
	"maket/internal/scene"
	"maket/internal/timeline"
)

type Config struct {
	Voice        string `json:"voice"`
	TTSPython    string `json:"tts_python"`
	StressPython string `json:"stress_python"`
	TTSModel     string `json:"tts_model"`
	RUAccentDir  string `json:"ruaccent_dir"`
	WhisperModel string `json:"whisper_model"`
}

type Spec struct {
	RefAudio string            `json:"ref_audio"`
	RefText  string            `json:"ref_text"`
	Stress   map[string]string `json:"stress"`
	Spell    map[string]string `json:"spell"`
}

type Result struct {
	Track    string                `json:"track"`
	Phrases  []timeline.PhraseTime `json:"phrases"`
	Warnings []string              `json:"warnings"`
}

const (
	headroom   = 0.25
	tailroom   = 0.35
	silenceDB  = -38.0
	frameSec   = 0.01
	maxRetries = 1
	tempoTol   = 0.10
	tempoMin   = 0.8
	tempoMax   = 1.25
)

func LoadConfig(root string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(root, "maket.json"))
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("maket.json: %w", err)
	}
	return c, nil
}

type run struct {
	ctx   context.Context
	root  string
	cfg   Config
	sc    *scene.Scene
	spec  Spec
	ref   string
	dir   string
	words []sceneWord
}

func Synthesize(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string) (*Result, error) {
	if name == "" {
		name = cfg.Voice
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	vdir := filepath.Join(root, "voices", name)
	specBytes, err := os.ReadFile(filepath.Join(vdir, "voice.json"))
	if err != nil {
		return nil, fmt.Errorf("voice %q: %w", name, err)
	}
	var spec Spec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return nil, fmt.Errorf("voice %q: %w", name, err)
	}
	r := &run{ctx: ctx, root: root, cfg: cfg, sc: sc, spec: spec, ref: filepath.Join(vdir, spec.RefAudio)}
	key, err := r.key(specBytes, nil)
	if err != nil {
		return nil, err
	}
	r.dir = filepath.Join(root, ".cache", "voice", key)
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, err
	}
	marked, err := r.stress()
	if err != nil {
		return nil, err
	}
	key, err = r.key(specBytes, marked)
	if err != nil {
		return nil, err
	}
	r.dir = filepath.Join(root, ".cache", "voice", key)
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, err
	}
	return r.synthesize(marked)
}

func (r *run) key(specBytes []byte, marked []string) (string, error) {
	h := sha256.New()
	ref, err := os.ReadFile(r.ref)
	if err != nil {
		return "", fmt.Errorf("voice sample: %w", err)
	}
	h.Write(ref)
	h.Write(specBytes)
	fmt.Fprintln(h, r.cfg.TTSModel, r.sc.Seed)
	for _, f := range []string{"stress.py", "tts.py", "asr.py"} {
		b, err := os.ReadFile(filepath.Join(r.root, "tools", "voice", f))
		if err != nil {
			return "", err
		}
		h.Write(b)
	}
	for _, ph := range r.sc.Phrases {
		p, _ := json.Marshal([]any{ph.Text, ph.Stress})
		h.Write(p)
	}
	if marked != nil {
		p, _ := json.Marshal(marked)
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil))[:24], nil
}

type piece struct {
	pcm   []float32
	from  float64
	to    float64
	words []sceneWord
	wav   string
}

func (r *run) synthesize(marked []string) (*Result, error) {
	for i, p := range r.sc.Phrases {
		for _, w := range scene.Words(p.Text) {
			r.words = append(r.words, sceneWord{text: w, phrase: i})
		}
	}
	main := filepath.Join(r.dir, "narration.wav")
	if err := r.tts([]ttsItem{{Text: strings.Join(marked, " "), Out: main}}, r.sc.Seed); err != nil {
		return nil, err
	}
	got, err := r.asr(main)
	if err != nil {
		return nil, err
	}
	align(r.words, got, r.spec.Spell)
	mainPCM, err := audio.Read(main)
	if err != nil {
		return nil, err
	}
	pieces := make([]*piece, len(r.sc.Phrases))
	var lost []int
	for i := range r.sc.Phrases {
		if missing(r.words, i) {
			lost = append(lost, i)
		}
	}
	r.cutMain(mainPCM, pieces, lost)
	for attempt := 0; len(lost) > 0 && attempt <= maxRetries; attempt++ {
		lost, err = r.regenerate(marked, pieces, lost, attempt)
		if err != nil {
			return nil, err
		}
	}
	if len(lost) > 0 {
		return nil, fmt.Errorf("voice did not say phrase %d «%s»", lost[0]+1, r.sc.Phrases[lost[0]].Text)
	}
	if err := r.fitTempo(pieces); err != nil {
		return nil, err
	}
	return r.place(pieces)
}

func (r *run) stress() ([]string, error) {
	var texts []string
	for _, p := range r.sc.Phrases {
		texts = append(texts, p.Text)
	}
	out := filepath.Join(r.dir, "stress.json")
	if _, err := os.Stat(out); err == nil {
		return r.readStress(out)
	}
	custom := map[string]string{}
	for k, v := range r.spec.Stress {
		if strings.Contains(v, "\u0301") {
			custom[k] = plusForm(v)
		}
	}
	job := map[string]any{"phrases": texts, "workdir": r.cfg.RUAccentDir, "custom_dict": custom, "out": out}
	if err := r.python(r.cfg.StressPython, "stress.py", job); err != nil {
		return nil, err
	}
	return r.readStress(out)
}

func (r *run) readStress(out string) ([]string, error) {
	var res struct {
		Marked []string `json:"marked"`
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	for i := range res.Marked {
		res.Marked[i] = restress(res.Marked[i], r.spec.Stress, r.sc.Phrases[i].Stress)
	}
	return res.Marked, nil
}

func plusForm(w string) string {
	r := []rune(w)
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		if i+1 < len(r) && r[i+1] == '\u0301' {
			b.WriteRune('+')
			b.WriteRune(r[i])
			i++
			continue
		}
		b.WriteRune(r[i])
	}
	return b.String()
}

func restress(text string, maps ...map[string]string) string {
	fields := strings.Fields(text)
	for i, f := range fields {
		core := strings.TrimFunc(f, func(c rune) bool { return !unicode.IsLetter(c) && c != '\u0301' })
		plain := scene.Fold(strings.ReplaceAll(core, "\u0301", ""))
		for _, m := range maps {
			if v, ok := m[plain]; ok && core != "" {
				fields[i] = strings.Replace(f, core, v, 1)
			}
		}
	}
	return strings.Join(fields, " ")
}

type ttsItem struct {
	Text string `json:"text"`
	Out  string `json:"out"`
}

func (r *run) tts(items []ttsItem, seed int) error {
	var todo []ttsItem
	for _, it := range items {
		if _, err := os.Stat(it.Out); err != nil {
			todo = append(todo, it)
		}
	}
	if len(todo) == 0 {
		return nil
	}
	items = todo
	job := map[string]any{"model": r.cfg.TTSModel, "ref_audio": r.ref, "ref_text": r.spec.RefText, "seed": seed, "items": items}
	return r.python(r.cfg.TTSPython, "tts.py", job)
}

func (r *run) asr(wav string) ([]heard, error) {
	out := strings.TrimSuffix(wav, ".wav") + ".words.json"
	if b, err := os.ReadFile(out); err == nil {
		var got []heard
		return got, json.Unmarshal(b, &got)
	}
	job := map[string]any{"model": r.cfg.WhisperModel, "items": []map[string]string{{"wav": wav, "out": out}}}
	if err := r.python(r.cfg.TTSPython, "asr.py", job); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	var got []heard
	return got, json.Unmarshal(b, &got)
}

func (r *run) python(py, script string, job any) error {
	b, _ := json.Marshal(job)
	jobPath := filepath.Join(r.dir, strings.TrimSuffix(script, ".py")+".job.json")
	if err := os.WriteFile(jobPath, b, 0o644); err != nil {
		return err
	}
	cmd := exec.CommandContext(r.ctx, py, "-I", filepath.Join(r.root, "tools", "voice", script), jobPath)
	cmd.Env = append(os.Environ(), "HF_HUB_OFFLINE=1")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", script, err)
	}
	return nil
}

func (r *run) phraseWords(i int) []sceneWord {
	var out []sceneWord
	for _, w := range r.words {
		if w.phrase == i {
			out = append(out, w)
		}
	}
	return out
}

func (r *run) cutMain(pcm []float32, pieces []*piece, lost []int) {
	fillTimes(r.words)
	isLost := map[int]bool{}
	for _, i := range lost {
		isLost[i] = true
	}
	var present []int
	for i := range r.sc.Phrases {
		if !isLost[i] {
			present = append(present, i)
		}
	}
	length := float64(len(pcm)) / audio.Rate
	cuts := make([]float64, len(present)+1)
	cuts[len(present)] = length
	for k := 1; k < len(present); k++ {
		a, b := r.phraseWords(present[k-1]), r.phraseWords(present[k])
		cuts[k] = pauseCut(pcm, a[len(a)-1].s, b[0].e, a[len(a)-1].e, b[0].s)
	}
	for k, i := range present {
		ws := r.phraseWords(i)
		from := math.Max(cuts[k], ws[0].s-headroom)
		to := math.Min(cuts[k+1], ws[len(ws)-1].e+tailroom)
		pieces[i] = &piece{pcm: pcm, from: from, to: to, words: ws}
	}
}

func (r *run) regenerate(marked []string, pieces []*piece, lost []int, attempt int) ([]int, error) {
	var items []ttsItem
	for _, i := range lost {
		items = append(items, ttsItem{Text: marked[i], Out: filepath.Join(r.dir, fmt.Sprintf("phrase%02d-%d.wav", i+1, attempt))})
	}
	if err := r.tts(items, r.sc.Seed+attempt+1); err != nil {
		return nil, err
	}
	var still []int
	for k, i := range lost {
		got, err := r.asr(items[k].Out)
		if err != nil {
			return nil, err
		}
		var ws []sceneWord
		for _, w := range scene.Words(r.sc.Phrases[i].Text) {
			ws = append(ws, sceneWord{text: w, phrase: i})
		}
		align(ws, got, r.spec.Spell)
		if missing(ws, i) {
			still = append(still, i)
			continue
		}
		fillTimes(ws)
		pcm, err := audio.Read(items[k].Out)
		if err != nil {
			return nil, err
		}
		length := float64(len(pcm)) / audio.Rate
		pieces[i] = &piece{pcm: pcm, from: math.Max(0, ws[0].s-headroom), to: math.Min(length, ws[len(ws)-1].e+tailroom), words: ws, wav: items[k].Out}
	}
	return still, nil
}

func (r *run) fitTempo(pieces []*piece) error {
	rate := make([]float64, len(pieces))
	var main []float64
	for i, p := range pieces {
		rate[i] = phraseTempo(p.words)
		if p.wav == "" {
			main = append(main, rate[i])
		}
	}
	mid := median(main)
	for i, p := range pieces {
		if p.wav == "" || rate[i] == 0 {
			continue
		}
		ref := neighborTempo(pieces, rate, i)
		if ref == 0 {
			ref = mid
		}
		if ref == 0 || math.Abs(rate[i]-ref)/ref <= tempoTol {
			continue
		}
		k := math.Min(tempoMax, math.Max(tempoMin, ref/rate[i]))
		pcm, err := audio.Read(p.wav, fmt.Sprintf("atempo=%g", k))
		if err != nil {
			return err
		}
		p.pcm = pcm
		p.from /= k
		p.to /= k
		for w := range p.words {
			p.words[w].s /= k
			p.words[w].e /= k
		}
	}
	return nil
}

func neighborTempo(pieces []*piece, rate []float64, i int) float64 {
	var sum float64
	n := 0
	if i > 0 && pieces[i-1].wav == "" && rate[i-1] > 0 {
		sum += rate[i-1]
		n++
	}
	if i+1 < len(pieces) && pieces[i+1].wav == "" && rate[i+1] > 0 {
		sum += rate[i+1]
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func phraseTempo(words []sceneWord) float64 {
	if len(words) == 0 {
		return 0
	}
	d := words[len(words)-1].e - words[0].s
	if d <= 0 {
		return 0
	}
	n := 0
	for _, w := range words {
		n += vowels(w.text)
	}
	return float64(n) / d
}

func vowels(w string) int {
	n := 0
	for _, r := range strings.ToLower(w) {
		if strings.ContainsRune("аеёиоуыэюя", r) {
			n++
		}
	}
	return n
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return s[m]
	}
	return (s[m-1] + s[m]) / 2
}

func pauseCut(pcm []float32, lo, hi, lastEnd, nextStart float64) float64 {
	frame := int(frameSec * audio.Rate)
	th := math.Pow(10, silenceDB/20)
	bestLen, bestMid := 0, -1.0
	runStart := -1
	a, b := int(lo*audio.Rate)/frame, int(hi*audio.Rate)/frame
	for f := a; f <= b; f++ {
		quiet := false
		if f >= 0 && (f+1)*frame <= len(pcm) {
			sum := 0.0
			for _, v := range pcm[f*frame : (f+1)*frame] {
				sum += float64(v) * float64(v)
			}
			quiet = math.Sqrt(sum/float64(frame)) < th
		}
		if quiet && runStart < 0 {
			runStart = f
		}
		if (!quiet || f == b) && runStart >= 0 {
			end := f
			if quiet {
				end = f + 1
			}
			if end-runStart > bestLen {
				bestLen = end - runStart
				bestMid = float64(runStart+end) / 2 * frameSec
			}
			runStart = -1
		}
	}
	if bestMid >= 0 {
		return bestMid
	}
	return (lastEnd + nextStart) / 2
}

func (r *run) place(pieces []*piece) (*Result, error) {
	res := &Result{Track: filepath.Join(r.dir, "voice.wav")}
	var track []float32
	t := timeline.Lead
	for i, p := range pieces {
		t += r.sc.Phrases[i].Pause
		a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
		at := int(t * audio.Rate)
		if need := at + (b - a); need > len(track) {
			track = append(track, make([]float32, need-len(track))...)
		}
		copy(track[at:], p.pcm[a:b])
		pt := timeline.PhraseTime{}
		var unheard []string
		for _, w := range p.words {
			pt.Words = append(pt.Words, timeline.WordTime{Text: w.text, S: t + w.s - p.from, E: t + w.e - p.from})
			if w.match < 0 {
				unheard = append(unheard, w.text)
			}
		}
		pt.Words[0].S = math.Max(pt.Words[0].S, t)
		pt.Start, pt.End = t, pt.Words[len(pt.Words)-1].E
		res.Phrases = append(res.Phrases, pt)
		if len(unheard) > 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("phrase %d: not heard: %s", i+1, strings.Join(unheard, ", ")))
		}
		t += p.to - p.from
	}
	return res, audio.Write(res.Track, track)
}
