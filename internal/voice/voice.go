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
	"unicode/utf8"

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
	RUAccent     bool   `json:"ruaccent"`
	WhisperModel string `json:"whisper_model"`
	ChecksPython string `json:"checks_python"`
	ChecksModel  string `json:"checks_model"`
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
	Tempo    []string              `json:"tempo"`
}

const (
	headroom     = 0.25
	tailroom     = 0.35
	silenceDB    = -38.0
	frameSec     = 0.01
	maxRetries   = 1
	edgeSec      = 0.08
	nearCut      = 0.15
	tempoTol     = 0.10
	tempoSnap    = 0.03
	takes        = 3
	takeSeedStep = 100
	takeTol      = 0.15
	choppyMax    = 0.3
	gapMin       = 0.5
	flatFall     = -3.0
	timbreRef    = 0.30
	timbreNear   = 0.40
	otherSpeaker = "voice sounds like another speaker"
	levelSnap    = 1.0
	levelTries   = 2
	peakMax      = 0.89
	bridgeSec    = 0.06
	raggedCut    = "voice piece is cut mid-word or has extra sound at its edges"
	repeatedAs   = "voice repeated «%s»"
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

func newRun(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string) (*run, []byte, error) {
	if name == "" {
		name = cfg.Voice
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	vdir := filepath.Join(root, "voices", name)
	specBytes, err := os.ReadFile(filepath.Join(vdir, "voice.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("voice %q: %w", name, err)
	}
	var spec Spec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return nil, nil, fmt.Errorf("voice %q: %w", name, err)
	}
	r := &run{ctx: ctx, root: root, cfg: cfg, sc: sc, spec: spec, ref: filepath.Join(vdir, spec.RefAudio)}
	key, err := r.key(specBytes, nil)
	if err != nil {
		return nil, nil, err
	}
	r.dir = filepath.Join(root, ".cache", "voice", key)
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, nil, err
	}
	return r, specBytes, nil
}

func Mark(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string) ([]string, error) {
	r, _, err := newRun(ctx, root, cfg, sc, name)
	if err != nil {
		return nil, err
	}
	return r.stress()
}

func Synthesize(ctx context.Context, root string, cfg Config, sc *scene.Scene, name string) (*Result, error) {
	r, specBytes, err := newRun(ctx, root, cfg, sc, name)
	if err != nil {
		return nil, err
	}
	marked, err := r.stress()
	if err != nil {
		return nil, err
	}
	key, err := r.key(specBytes, marked)
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
	seed  int
}

type take struct {
	words  []sceneWord
	pieces []*piece
	lost   []int
	why    map[int]string
	kept   map[int]*piece
	spread float64
	choppy float64
	flat   int
}

func (r *run) synthesize(marked []string) (*Result, error) {
	if r.cfg.ChecksPython != "" {
		for _, p := range []string{r.cfg.ChecksPython, r.cfg.ChecksModel} {
			if _, err := os.Stat(p); err != nil {
				return nil, fmt.Errorf("voice checks in maket.json (checks_python, checks_model): %w", err)
			}
		}
	}
	var best *take
	for t := 0; t < takes; t++ {
		tk, err := r.record(marked, r.sc.Seed+t*takeSeedStep)
		if err != nil {
			return nil, err
		}
		if best == nil || len(tk.lost) < len(best.lost) || len(tk.lost) == len(best.lost) && (tk.choppy < best.choppy || tk.choppy == best.choppy && (tk.flat < best.flat || tk.flat == best.flat && tk.spread < best.spread)) {
			best = tk
		}
		if len(best.lost) == 0 && best.choppy <= choppyMax && best.spread <= takeTol {
			break
		}
	}
	r.words = best.words
	pieces, lost, why, kept := best.pieces, best.lost, best.why, best.kept
	var err error
	keptWhy := map[int]string{}
	for i := range kept {
		keptWhy[i] = why[i]
	}
	for attempt := 0; len(lost) > 0 && attempt <= maxRetries; attempt++ {
		lost, err = r.regenerate(marked, pieces, lost, attempt, why)
		if err != nil {
			return nil, err
		}
	}
	var warnings []string
	var still []int
	for _, i := range lost {
		if kept[i] != nil {
			pieces[i] = kept[i]
			warnings = append(warnings, fmt.Sprintf("phrase %d: %s", i+1, keptWhy[i]))
			continue
		}
		still = append(still, i)
	}
	lost = still
	if len(lost) > 0 {
		if w := why[lost[0]]; w != "" {
			return nil, fmt.Errorf("phrase %d: %s", lost[0]+1, w)
		}
		return nil, fmt.Errorf("voice did not say phrase %d «%s»", lost[0]+1, r.sc.Phrases[lost[0]].Text)
	}
	tempo, err := r.evenPieces(pieces)
	if err != nil {
		return nil, err
	}
	res, err := r.place(pieces)
	if err != nil {
		return nil, err
	}
	res.Warnings = append(res.Warnings, warnings...)
	for i, p := range pieces {
		if p.wav != "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("phrase %d: recorded apart from the main take (seed %d), may sound like another voice", i+1, p.seed))
		}
	}
	if r.cfg.ChecksPython == "" {
		res.Warnings = append(res.Warnings, "voice checks are off: set checks_python and checks_model in maket.json")
	}
	res.Tempo = append([]string{fmt.Sprintf("take: short pauses %.2f/s of speech, tempo spread %.0f%%", best.choppy, best.spread*100)}, tempo...)
	return res, nil
}

func (r *run) record(marked []string, seed int) (*take, error) {
	r.words = nil
	for i, p := range r.sc.Phrases {
		for _, w := range scene.Words(p.Text) {
			r.words = append(r.words, sceneWord{text: w, phrase: i})
		}
	}
	main := filepath.Join(r.dir, "narration.wav")
	if seed != r.sc.Seed {
		main = filepath.Join(r.dir, fmt.Sprintf("narration-%d.wav", seed))
	}
	if err := r.tts([]ttsItem{{Text: strings.Join(marked, " "), Out: main}}, seed); err != nil {
		return nil, err
	}
	got, err := r.asr(main)
	if err != nil {
		return nil, err
	}
	owner := align(r.words, got, r.spec.Spell)
	mainPCM, err := audio.Read(main)
	if err != nil {
		return nil, err
	}
	pieces := make([]*piece, len(r.sc.Phrases))
	var lost []int
	why := map[int]string{}
	for i := range r.sc.Phrases {
		if w := repeated(r.words, got, owner, r.spec.Spell, i); w != "" {
			lost = append(lost, i)
			why[i] = fmt.Sprintf(repeatedAs, w)
			continue
		}
		if missing(r.words, i) {
			lost = append(lost, i)
		}
	}
	r.cutMain(mainPCM, pieces, lost, seed)
	dirty, err := r.dirty(pieces)
	if err != nil {
		return nil, err
	}
	kept := map[int]*piece{}
	for _, i := range dirty {
		kept[i] = pieces[i]
		pieces[i] = nil
		lost = append(lost, i)
		why[i] = raggedCut
	}
	cs, err := r.checks(pieces)
	if err != nil {
		return nil, err
	}
	flat := 0
	for i, c := range cs {
		if c == nil {
			continue
		}
		if c.otherVoice() {
			kept[i] = pieces[i]
			pieces[i] = nil
			lost = append(lost, i)
			why[i] = otherSpeaker
			continue
		}
		if r.flat(i, c) {
			flat++
		}
	}
	sort.Ints(lost)
	return &take{words: r.words, pieces: pieces, lost: lost, why: why, kept: kept, spread: r.spread(pieces), choppy: choppy(pieces), flat: flat}, nil
}

func quietFrames(p *piece) []bool {
	frame := int(frameSec * audio.Rate)
	th := math.Pow(10, silenceDB/20)
	a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
	var q []bool
	for f := a; f+frame <= b; f += frame {
		e := 0.0
		for _, v := range p.pcm[f : f+frame] {
			e += float64(v) * float64(v)
		}
		q = append(q, math.Sqrt(e/float64(frame)) < th)
	}
	return q
}

func choppy(pieces []*piece) float64 {
	short, speech := 0, 0
	for _, p := range pieces {
		if p == nil {
			continue
		}
		q := quietFrames(p)
		run, started := 0, false
		for _, quiet := range q {
			if quiet {
				run++
				continue
			}
			if started && float64(run)*frameSec >= 0.12 && float64(run)*frameSec < 0.25 {
				short++
			}
			speech++
			run, started = 0, true
		}
	}
	if speech == 0 {
		return 0
	}
	return float64(short) / (float64(speech) * frameSec)
}

type check struct {
	Ref  float64  `json:"ref"`
	Prev *float64 `json:"prev"`
	Next *float64 `json:"next"`
	Fall float64  `json:"fall"`
}

func (r *run) checks(pieces []*piece) ([]*check, error) {
	out := make([]*check, len(pieces))
	if r.cfg.ChecksPython == "" {
		return out, nil
	}
	var idx []int
	var wavs []string
	for i, p := range pieces {
		if p == nil {
			continue
		}
		a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
		wav := filepath.Join(r.dir, fmt.Sprintf("check%02d-%d-%d-%d-%d.wav", i+1, p.seed, a, b, len(p.pcm)))
		if _, err := os.Stat(wav); err != nil {
			if err := audio.Write(wav, p.pcm[a:b]); err != nil {
				return nil, err
			}
		}
		idx = append(idx, i)
		wavs = append(wavs, wav)
	}
	if len(wavs) == 0 {
		return out, nil
	}
	h := sha256.Sum256([]byte(strings.Join(wavs, "\n")))
	res := filepath.Join(r.dir, "checks-"+hex.EncodeToString(h[:])[:16]+".json")
	if _, err := os.Stat(res); err != nil {
		job := map[string]any{"model": r.cfg.ChecksModel, "ref": r.ref, "wavs": wavs, "out": res}
		if err := r.python(r.cfg.ChecksPython, "checks.py", job); err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(res)
	if err != nil {
		return nil, err
	}
	var got []*check
	if err := json.Unmarshal(b, &got); err != nil {
		return nil, err
	}
	for k, i := range idx {
		out[i] = got[k]
	}
	return out, nil
}

func (c *check) otherVoice() bool {
	near := 0.0
	for _, v := range []*float64{c.Prev, c.Next} {
		if v != nil {
			near = math.Max(near, *v)
		}
	}
	return (c.Prev != nil || c.Next != nil) && c.Ref < timbreRef && near < timbreNear
}

func (r *run) flat(i int, c *check) bool {
	return c != nil && c.Fall != 0 && strings.HasSuffix(strings.TrimSpace(r.sc.Phrases[i].Text), ".") && c.Fall > flatFall
}

func edgeQuiet(p *piece, tail bool) float64 {
	q := quietFrames(p)
	n := 0
	for k := range q {
		j := k
		if tail {
			j = len(q) - 1 - k
		}
		if !q[j] {
			break
		}
		n++
	}
	return float64(n) * frameSec
}

func (r *run) spread(pieces []*piece) float64 {
	var rates []float64
	for i, p := range pieces {
		if p != nil {
			rates = append(rates, r.speechRate(i, p))
		}
	}
	m := median(rates)
	worst := 0.0
	for _, v := range rates {
		if m > 0 && v > 0 {
			worst = math.Max(worst, math.Abs(v/m-1))
		}
	}
	return worst
}

func (r *run) stress() ([]string, error) {
	var texts []string
	for _, p := range r.sc.Phrases {
		texts = append(texts, p.Text)
	}
	if !r.cfg.RUAccent {
		return r.restressAll(texts), nil
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
	return r.restressAll(res.Marked), nil
}

func (r *run) restressAll(texts []string) []string {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = restress(t, r.spec.Stress, r.sc.Phrases[i].Stress)
	}
	return out
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
				if c, _ := utf8.DecodeRuneInString(core); unicode.IsUpper(c) {
					first, n := utf8.DecodeRuneInString(v)
					v = string(unicode.ToUpper(first)) + v[n:]
				}
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
	got, err := r.asrAll([]string{wav})
	if err != nil {
		return nil, err
	}
	return got[0], nil
}

func (r *run) asrAll(wavs []string) ([][]heard, error) {
	outs := make([]string, len(wavs))
	var items []map[string]string
	for k, wav := range wavs {
		outs[k] = strings.TrimSuffix(wav, ".wav") + ".words.json"
		if _, err := os.Stat(outs[k]); err != nil {
			items = append(items, map[string]string{"wav": wav, "out": outs[k]})
		}
	}
	if len(items) > 0 {
		job := map[string]any{"model": r.cfg.WhisperModel, "items": items}
		if err := r.python(r.cfg.TTSPython, "asr.py", job); err != nil {
			return nil, err
		}
	}
	all := make([][]heard, len(wavs))
	for k, out := range outs {
		b, err := os.ReadFile(out)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &all[k]); err != nil {
			return nil, err
		}
	}
	return all, nil
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

func (r *run) cutMain(pcm []float32, pieces []*piece, lost []int, seed int) {
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
		cuts[k] = pauseCut(pcm, a[len(a)-1].e-nearCut, b[0].e, a[len(a)-1].e, b[0].s)
	}
	for k, i := range present {
		ws := r.phraseWords(i)
		from := math.Max(cuts[k], ws[0].s-headroom)
		to := math.Min(cuts[k+1], ws[len(ws)-1].e+tailroom)
		pieces[i] = &piece{pcm: pcm, from: from, to: to, words: ws, seed: seed}
	}
}

func (r *run) dirty(pieces []*piece) ([]int, error) {
	var idx []int
	var wavs []string
	for i, p := range pieces {
		if p == nil {
			continue
		}
		a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
		wav := filepath.Join(r.dir, fmt.Sprintf("piece%02d-%d-%d-%d.wav", i+1, p.seed, a, b))
		if _, err := os.Stat(wav); err != nil {
			if err := audio.Write(wav, p.pcm[a:b]); err != nil {
				return nil, err
			}
		}
		idx = append(idx, i)
		wavs = append(wavs, wav)
	}
	got, err := r.asrAll(wavs)
	if err != nil {
		return nil, err
	}
	var out []int
	for k, i := range idx {
		p := pieces[i]
		ws := r.sceneWords(i)
		owner := align(ws, got[k], r.spec.Spell)
		lostFirst := p.words[0].match >= 0 && ws[0].match < 0 && utf8.RuneCountInString(ws[0].text) > 2
		if lostFirst || !quietEdges(p.pcm, p.from, p.to, p.words) || ragged(ws, owner) {
			out = append(out, i)
		}
	}
	return out, nil
}

func (r *run) sceneWords(i int) []sceneWord {
	var ws []sceneWord
	for _, w := range scene.Words(r.sc.Phrases[i].Text) {
		ws = append(ws, sceneWord{text: w, phrase: i})
	}
	return ws
}

func quietEdges(pcm []float32, from, to float64, ws []sceneWord) bool {
	s, e := ws[0].s, ws[len(ws)-1].e
	if to < (ws[len(ws)-1].s+e)/2 {
		return false
	}
	th := math.Pow(10, silenceDB/20)
	level := func(a, b float64) float64 {
		i, j := max(0, int(a*audio.Rate)), min(len(pcm), int(b*audio.Rate))
		if j <= i {
			return 0
		}
		sum := 0.0
		for _, v := range pcm[i:j] {
			sum += float64(v) * float64(v)
		}
		return math.Sqrt(sum / float64(j-i))
	}
	return level(from, math.Min(from+edgeSec, s)) < th && level(math.Max(to-edgeSec, e), to) < th
}

func ragged(ws []sceneWord, owner []int) bool {
	first, last := -1, -1
	for k, w := range ws {
		if w.match < 0 {
			continue
		}
		if first < 0 {
			first = k
		}
		last = k
	}
	if first < 0 || ws[len(ws)-1].match < 0 {
		return true
	}
	before, after := 0, 0
	for j, o := range owner {
		if o >= 0 {
			continue
		}
		if j < ws[first].match {
			before++
		}
		if j > ws[last].match {
			after++
		}
	}
	return before > first || after > len(ws)-1-last
}

func (r *run) regenerate(marked []string, pieces []*piece, lost []int, attempt int, why map[int]string) ([]int, error) {
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
		ws := r.sceneWords(i)
		owner := align(ws, got, r.spec.Spell)
		if w := repeated(ws, got, owner, r.spec.Spell, i); w != "" {
			why[i] = fmt.Sprintf(repeatedAs, w)
			still = append(still, i)
			continue
		}
		delete(why, i)
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
		from, to := math.Max(0, ws[0].s-headroom), math.Min(length, ws[len(ws)-1].e+tailroom)
		if !quietEdges(pcm, from, to, ws) || ragged(ws, owner) {
			why[i] = raggedCut
			still = append(still, i)
			continue
		}
		pieces[i] = &piece{pcm: pcm, from: from, to: to, words: ws, wav: items[k].Out, seed: r.sc.Seed + attempt + 1}
	}
	return still, nil
}

func (r *run) evenPieces(pieces []*piece) ([]string, error) {
	rate := make([]float64, len(pieces))
	var main, mainLevel, allLevel []float64
	for i, p := range pieces {
		rate[i] = r.speechRate(i, p)
		allLevel = append(allLevel, speechLevel(p))
		if p.wav == "" {
			main = append(main, rate[i])
			mainLevel = append(mainLevel, allLevel[i])
		}
	}
	level := median(mainLevel)
	if level == 0 {
		level = median(allLevel)
	}
	target := median(main)
	if target == 0 {
		target = median(rate)
	}
	lines := make([]string, len(pieces))
	for i, p := range pieces {
		k := 1.0
		if rate[i] > 0 && target > 0 && math.Abs(rate[i]/target-1) > tempoSnap {
			k = math.Min(1+tempoTol, math.Max(1-tempoTol, target/rate[i]))
			if err := r.stretch(i, p, k); err != nil {
				return nil, err
			}
		}
		was := speechLevel(p)
		for try := 0; try < levelTries; try++ {
			if v := speechLevel(p); v > 0 && level > 0 {
				if g := math.Min(level/v, peakMax/peak(p)); math.Abs(db(g)) > levelSnap {
					p.gain(g)
				}
			}
		}
		src := "main"
		if p.wav != "" {
			src = "re-voiced"
		}
		lines[i] = fmt.Sprintf("phrase %d: %s seed %d, %.1f → %.1f syl/s, atempo %.2f, target %.1f; speech %.1f → %.1f dB, target %.1f", i+1, src, p.seed, rate[i], rate[i]*k, k, target, db(was), db(speechLevel(p)), db(level))
	}
	final, err := r.checks(pieces)
	if err != nil {
		return nil, err
	}
	for i, p := range pieces {
		if c := final[i]; c != nil {
			lines[i] += fmt.Sprintf("; voice %.2f to sample", c.Ref)
			if c.Prev != nil {
				lines[i] += fmt.Sprintf(", %.2f to previous", *c.Prev)
			}
			lines[i] += fmt.Sprintf(", end fall %+.1f st", c.Fall)
		}
		if i+1 < len(pieces) {
			lines[i] += fmt.Sprintf(", pause after %.2f s", edgeQuiet(p, true)+edgeQuiet(pieces[i+1], false))
		}
		if r.flat(i, final[i]) {
			lines[i] += ", FLAT END"
		}
		if final[i] != nil && final[i].otherVoice() {
			lines[i] += ", OTHER VOICE"
		}
	}
	return lines, nil
}

func (r *run) speechRate(i int, p *piece) float64 {
	n := 0
	for _, w := range scene.Words(r.sc.Phrases[i].Text) {
		n += vowels(w)
	}
	frame := int(frameSec * audio.Rate)
	th := math.Pow(10, silenceDB/20)
	a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
	voiced, gap, started := 0, 0, false
	for f := a; f+frame <= b; f += frame {
		sum := 0.0
		for _, v := range p.pcm[f : f+frame] {
			sum += float64(v) * float64(v)
		}
		if math.Sqrt(sum/float64(frame)) < th {
			gap++
			continue
		}
		if started && float64(gap)*frameSec <= bridgeSec {
			voiced += gap
		}
		voiced++
		gap, started = 0, true
	}
	if voiced == 0 {
		return 0
	}
	return float64(n) / (float64(voiced) * frameSec)
}

func speechLevel(p *piece) float64 {
	frame := int(frameSec * audio.Rate)
	th := math.Pow(10, silenceDB/20)
	a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
	sum, n := 0.0, 0
	for f := a; f+frame <= b; f += frame {
		e := 0.0
		for _, v := range p.pcm[f : f+frame] {
			e += float64(v) * float64(v)
		}
		if math.Sqrt(e/float64(frame)) >= th {
			sum += e
			n += frame
		}
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(n))
}

func peak(p *piece) float64 {
	a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
	m := 0.0
	for _, v := range p.pcm[a:b] {
		m = math.Max(m, math.Abs(float64(v)))
	}
	return m
}

func db(v float64) float64 {
	return 20 * math.Log10(v)
}

func (p *piece) gain(g float64) {
	a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
	pcm := make([]float32, b-a)
	for j, v := range p.pcm[a:b] {
		pcm[j] = v * float32(g)
	}
	for w := range p.words {
		p.words[w].s -= p.from
		p.words[w].e -= p.from
	}
	p.pcm, p.from, p.to = pcm, 0, float64(len(pcm))/audio.Rate
}

func (r *run) stretch(i int, p *piece, k float64) error {
	a, b := int(p.from*audio.Rate), min(len(p.pcm), int(p.to*audio.Rate))
	wav := filepath.Join(r.dir, fmt.Sprintf("tempo%02d-%d-%d-%d.wav", i+1, p.seed, a, b))
	if _, err := os.Stat(wav); err != nil {
		if err := audio.Write(wav, p.pcm[a:b]); err != nil {
			return err
		}
	}
	pcm, err := audio.Read(wav, fmt.Sprintf("atempo=%g", k))
	if err != nil {
		return err
	}
	for w := range p.words {
		p.words[w].s = (p.words[w].s - p.from) / k
		p.words[w].e = (p.words[w].e - p.from) / k
	}
	p.pcm, p.from, p.to = pcm, 0, float64(len(pcm))/audio.Rate
	return nil
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
		if i > 0 {
			if gap := edgeQuiet(pieces[i-1], true) + edgeQuiet(p, false) + r.sc.Phrases[i].Pause; gap < gapMin {
				t += gapMin - gap
			}
		}
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
			if w.match < 0 || w.guess {
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
