package voice

import (
	"strings"
	"unicode/utf8"

	"maket/internal/scene"
)

type heard struct {
	W string  `json:"w"`
	S float64 `json:"s"`
	E float64 `json:"e"`
}

type sceneWord struct {
	text   string
	phrase int
	match  int
	s, e   float64
}

func norm(w string, spell map[string]string) string {
	w = scene.Fold(strings.Trim(w, ".,!?:;«»\"—()…-"))
	if v, ok := spell[w]; ok {
		return scene.Fold(v)
	}
	if v, ok := numbers[w]; ok {
		return v
	}
	return translit(w)
}

var numbers = map[string]string{
	"0": "ноль", "1": "один", "2": "два", "3": "три", "4": "четыре", "5": "пять", "6": "шесть", "7": "семь",
	"8": "восемь", "9": "девять", "10": "десять", "20": "двадцать", "30": "тридцать", "40": "сорок",
	"100": "сто",
}

var latin = map[rune]string{
	'a': "а", 'b': "б", 'c': "к", 'd': "д", 'e': "е", 'f': "ф", 'g': "г", 'h': "х", 'i': "и", 'j': "дж",
	'k': "к", 'l': "л", 'm': "м", 'n': "н", 'o': "о", 'p': "п", 'q': "к", 'r': "р", 's': "с", 't': "т",
	'u': "у", 'v': "в", 'w': "в", 'x': "кс", 'y': "й", 'z': "з",
}

func translit(w string) string {
	var b strings.Builder
	for _, r := range w {
		if v, ok := latin[r]; ok {
			b.WriteString(v)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func similar(a, b string) bool {
	if a == b {
		return true
	}
	n := max(utf8.RuneCountInString(a), utf8.RuneCountInString(b))
	return levenshtein(a, b)*3 <= n
}

func merged(first, second, heard string) bool {
	f := []rune(first)
	if len(f) < 3 || !strings.HasPrefix(heard, string(f[:3])) {
		return false
	}
	return levenshtein(first+second, heard) <= utf8.RuneCountInString(second)
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			c := 1
			if ra[i-1] == rb[j-1] {
				c = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+c)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

const (
	stepMatch = iota + 1
	stepMerge
	stepSkipScene
	stepSkipHeard
)

func align(words []sceneWord, got []heard, spell map[string]string) {
	n, m := len(words), len(got)
	sw := make([]string, n)
	for i, w := range words {
		sw[i] = norm(w.text, spell)
	}
	hw := make([]string, m)
	for j, h := range got {
		hw[j] = norm(h.W, spell)
	}
	cost := make([][]int, n+1)
	step := make([][]int, n+1)
	for i := range cost {
		cost[i] = make([]int, m+1)
		step[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		cost[i][0], step[i][0] = i, stepSkipScene
	}
	for j := 1; j <= m; j++ {
		cost[0][j], step[0][j] = j, stepSkipHeard
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			best, how := cost[i-1][j]+1, stepSkipScene
			if c := cost[i][j-1] + 1; c < best {
				best, how = c, stepSkipHeard
			}
			if similar(sw[i-1], hw[j-1]) {
				if c := cost[i-1][j-1]; c < best {
					best, how = c, stepMatch
				}
			}
			if i >= 2 && i < n && words[i-2].phrase == words[i-1].phrase && words[i].phrase == words[i-1].phrase && merged(sw[i-2], sw[i-1], hw[j-1]) {
				if c := cost[i-2][j-1]; c < best {
					best, how = c, stepMerge
				}
			}
			cost[i][j], step[i][j] = best, how
		}
	}
	for i := range words {
		words[i].match, words[i].s, words[i].e = -1, 0, 0
	}
	i, j := n, m
	for i > 0 || j > 0 {
		switch step[i][j] {
		case stepMatch:
			words[i-1].match = j - 1
			i, j = i-1, j-1
		case stepMerge:
			h := got[j-1]
			mid := h.S + (h.E-h.S)*float64(len([]rune(sw[i-2])))/float64(len([]rune(sw[i-2]+sw[i-1])))
			words[i-2].match, words[i-1].match = j-1, j-1
			words[i-2].s, words[i-2].e = h.S, mid
			words[i-1].s, words[i-1].e = mid, h.E
			i, j = i-2, j-1
		case stepSkipScene:
			i--
		default:
			j--
		}
	}
	for k := range words {
		if mt := words[k].match; mt >= 0 && words[k].s == 0 && words[k].e == 0 {
			words[k].s, words[k].e = got[mt].S, got[mt].E
		}
	}
}

func missing(words []sceneWord, phrase int) bool {
	total, found, last := 0, 0, -1
	for k, w := range words {
		if w.phrase != phrase {
			continue
		}
		total++
		if w.match >= 0 {
			found++
		}
		last = k
	}
	return total == 0 || words[last].match < 0 || found*2 < total
}

func fillTimes(words []sceneWord) {
	for k := 0; k < len(words); {
		if words[k].match >= 0 {
			k++
			continue
		}
		end := k
		for end < len(words) && words[end].match < 0 {
			end++
		}
		from, to := 0.0, 0.0
		if k > 0 {
			from = words[k-1].e
		}
		if end < len(words) {
			to = words[end].s
		} else {
			to = from + 0.3*float64(end-k)
		}
		letters := 0
		for _, w := range words[k:end] {
			letters += len([]rune(w.text)) + 1
		}
		t := from
		for x := k; x < end; x++ {
			d := (to - from) * float64(len([]rune(words[x].text))+1) / float64(max(1, letters))
			words[x].s, words[x].e = t, t+d
			t += d
		}
		k = end
	}
}
