package voice

import (
	"strconv"
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
	guess  bool
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

var numberWords = map[string]int{
	"ноль": 0, "нуль": 0, "нуля": 0, "нулю": 0, "нулем": 0, "нуле": 0,
	"один": 1, "одна": 1, "одно": 1, "одного": 1, "одной": 1, "одному": 1, "одну": 1, "одним": 1, "одною": 1, "одном": 1,
	"одни": 1, "одних": 1, "одними": 1,
	"два": 2, "две": 2, "двух": 2, "двум": 2, "двумя": 2,
	"три": 3, "трех": 3, "трем": 3, "тремя": 3,
	"четыре": 4, "четырех": 4, "четырем": 4, "четырьмя": 4,
	"пять": 5, "пяти": 5, "пятью": 5,
	"шесть": 6, "шести": 6, "шестью": 6,
	"семь": 7, "семи": 7, "семью": 7,
	"восемь": 8, "восьми": 8, "восемью": 8, "восьмью": 8,
	"девять": 9, "девяти": 9, "девятью": 9,
	"десять": 10, "десяти": 10, "десятью": 10,
	"одиннадцать": 11, "одиннадцати": 11, "одиннадцатью": 11,
	"двенадцать": 12, "двенадцати": 12, "двенадцатью": 12,
	"тринадцать": 13, "тринадцати": 13, "тринадцатью": 13,
	"четырнадцать": 14, "четырнадцати": 14, "четырнадцатью": 14,
	"пятнадцать": 15, "пятнадцати": 15, "пятнадцатью": 15,
	"шестнадцать": 16, "шестнадцати": 16, "шестнадцатью": 16,
	"семнадцать": 17, "семнадцати": 17, "семнадцатью": 17,
	"восемнадцать": 18, "восемнадцати": 18, "восемнадцатью": 18,
	"девятнадцать": 19, "девятнадцати": 19, "девятнадцатью": 19,
	"двадцать": 20, "двадцати": 20, "двадцатью": 20,
	"тридцать": 30, "тридцати": 30, "тридцатью": 30,
	"сорок": 40, "сорока": 40,
	"пятьдесят": 50, "пятидесяти": 50, "пятьюдесятью": 50,
	"шестьдесят": 60, "шестидесяти": 60, "шестьюдесятью": 60,
	"семьдесят": 70, "семидесяти": 70, "семьюдесятью": 70,
	"восемьдесят": 80, "восьмидесяти": 80, "восемьюдесятью": 80, "восьмьюдесятью": 80,
	"девяносто": 90, "девяноста": 90,
	"сто": 100, "ста": 100,
	"двести": 200, "двухсот": 200, "двумстам": 200, "двумястами": 200, "двухстах": 200,
	"триста": 300, "трехсот": 300, "тремстам": 300, "тремястами": 300, "трехстах": 300,
	"четыреста": 400, "четырехсот": 400, "четыремстам": 400, "четырьмястами": 400, "четырехстах": 400,
	"пятьсот": 500, "пятисот": 500, "пятистам": 500, "пятьюстами": 500, "пятистах": 500,
	"шестьсот": 600, "шестисот": 600, "шестистам": 600, "шестьюстами": 600, "шестистах": 600,
	"семьсот": 700, "семисот": 700, "семистам": 700, "семьюстами": 700, "семистах": 700,
	"восемьсот": 800, "восьмисот": 800, "восьмистам": 800, "восемьюстами": 800, "восьмьюстами": 800, "восьмистах": 800,
	"девятьсот": 900, "девятисот": 900, "девятистам": 900, "девятьюстами": 900, "девятистах": 900,
	"тысяча": 1000, "тысячи": 1000, "тысяч": 1000, "тысяче": 1000, "тысячу": 1000, "тысячей": 1000, "тысячею": 1000,
	"тысячам": 1000, "тысячами": 1000, "тысячах": 1000,
}

var ordinalWords = map[string]int{
	"нулевого": 0, "первого": 1, "второго": 2, "третьего": 3, "четвертого": 4, "пятого": 5,
	"шестого": 6, "седьмого": 7, "восьмого": 8, "девятого": 9, "десятого": 10,
	"одиннадцатого": 11, "двенадцатого": 12, "тринадцатого": 13, "четырнадцатого": 14, "пятнадцатого": 15,
	"шестнадцатого": 16, "семнадцатого": 17, "восемнадцатого": 18, "девятнадцатого": 19,
	"двадцатого": 20, "тридцатого": 30, "сорокового": 40, "пятидесятого": 50, "шестидесятого": 60,
	"семидесятого": 70, "восьмидесятого": 80, "девяностого": 90, "сотого": 100,
	"двухсотого": 200, "трехсотого": 300, "четырехсотого": 400, "пятисотого": 500,
	"шестисотого": 600, "семисотого": 700, "восьмисотого": 800, "девятисотого": 900, "тысячного": 1000,
}

var percentWords = map[string]bool{
	"процент": true, "процента": true, "процентов": true, "проценту": true, "процентом": true,
	"проценте": true, "проценты": true, "процентам": true, "процентами": true, "процентах": true,
}

type numberSpan struct {
	value  int
	count  int
	suffix string
}

func sceneNumbers(words []sceneWord, sw []string) [][]numberSpan {
	spans := make([][]numberSpan, len(words))
	for start := range words {
		total, group, limit := 0, 0, 1000
		for end := start; end < len(words) && words[end].phrase == words[start].phrase; end++ {
			w := sw[end]
			if _, ok := percentWords[w]; ok {
				if end > start {
					spans[end] = append(spans[end], numberSpan{total + group, end - start + 1, "%"})
				}
				break
			}
			v, ok := numberWords[w]
			suffix := ""
			if !ok {
				v, ok = ordinalWords[w]
				suffix = "-го"
			}
			if !ok || v == 0 && end != start {
				break
			}
			if v == 1000 {
				if total != 0 || end > start && group == 0 {
					break
				}
				total, group, limit = max(1, group)*1000, 0, 1000
			} else {
				if v >= limit {
					break
				}
				group += v
				switch {
				case v >= 100:
					limit = 100
				case v >= 20:
					limit = 10
				default:
					limit = 1
				}
			}
			spans[end] = append(spans[end], numberSpan{total + group, end - start + 1, suffix})
			if suffix != "" {
				break
			}
		}
	}
	return spans
}

func heardNumbers(got []heard) []numberSpan {
	spans := make([]numberSpan, len(got))
	for j, h := range got {
		w := scene.Fold(strings.Trim(h.W, ".,!?:;«»\"—()…"))
		if (w == "-го" || w == "%") && j > 0 && spans[j-1].count == 1 && spans[j-1].suffix == "" {
			spans[j] = numberSpan{spans[j-1].value, 2, w}
			continue
		}
		suffix := ""
		for _, tail := range []string{"-го", "%"} {
			if strings.HasSuffix(w, tail) {
				w, suffix = strings.TrimSuffix(w, tail), tail
				break
			}
		}
		if strings.Trim(w, "0123456789") != "" {
			continue
		}
		v, err := strconv.Atoi(w)
		if err == nil {
			spans[j] = numberSpan{v, 1, suffix}
		}
	}
	return spans
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

func alike(a, b string) bool {
	short := min(utf8.RuneCountInString(a), utf8.RuneCountInString(b))
	n := max(utf8.RuneCountInString(a), utf8.RuneCountInString(b))
	return short >= 4 && levenshtein(a, b)*2 <= n
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
	stepNumber
	stepSkipScene
	stepSkipHeard
	stepGuess
)

func align(words []sceneWord, got []heard, spell map[string]string) []int {
	n, m := len(words), len(got)
	owner := make([]int, m)
	for j := range owner {
		owner[j] = -1
	}
	sw := make([]string, n)
	for i, w := range words {
		sw[i] = norm(w.text, spell)
	}
	hw := make([]string, m)
	for j, h := range got {
		hw[j] = norm(h.W, spell)
	}
	sn, hn := sceneNumbers(words, sw), heardNumbers(got)
	cost := make([][]int, n+1)
	step := make([][]int, n+1)
	count := make([][]int, n+1)
	for i := range cost {
		cost[i] = make([]int, m+1)
		step[i] = make([]int, m+1)
		count[i] = make([]int, m+1)
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
			} else if alike(sw[i-1], hw[j-1]) {
				if c := cost[i-1][j-1] + 1; c < best {
					best, how = c, stepGuess
				}
			}
			if i >= 2 && i < n && words[i-2].phrase == words[i-1].phrase && words[i].phrase == words[i-1].phrase && merged(sw[i-2], sw[i-1], hw[j-1]) {
				if c := cost[i-2][j-1]; c < best {
					best, how = c, stepMerge
				}
			}
			if h := hn[j-1]; h.count > 0 {
				for _, s := range sn[i-1] {
					if s.value == h.value && s.suffix == h.suffix {
						if c := cost[i-s.count][j-h.count]; c < best {
							best, how, count[i][j] = c, stepNumber, s.count
						}
					}
				}
			}
			cost[i][j], step[i][j] = best, how
		}
	}
	for i := range words {
		words[i].match, words[i].guess, words[i].s, words[i].e = -1, false, 0, 0
	}
	i, j := n, m
	for i > 0 || j > 0 {
		switch step[i][j] {
		case stepMatch, stepGuess:
			words[i-1].match, words[i-1].guess = j-1, step[i][j] == stepGuess
			if step[i][j] == stepMatch {
				owner[j-1] = words[i-1].phrase
			}
			i, j = i-1, j-1
		case stepMerge, stepNumber:
			size, heardSize := 2, 1
			if step[i][j] == stepNumber {
				size, heardSize = count[i][j], hn[j-1].count
			}
			from, to := got[j-heardSize].S, got[j-1].E
			letters := 0
			for _, w := range sw[i-size : i] {
				letters += utf8.RuneCountInString(w)
			}
			t := from
			for k := i - size; k < i; k++ {
				d := (to - from) * float64(utf8.RuneCountInString(sw[k])) / float64(letters)
				words[k].match, words[k].s, words[k].e = j-heardSize, t, t+d
				t += d
			}
			for k := 0; k < heardSize; k++ {
				owner[j-heardSize+k] = words[i-1].phrase
			}
			i, j = i-size, j-heardSize
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
	return owner
}

func repeated(words []sceneWord, got []heard, owner []int, spell map[string]string, phrase int) string {
	count := map[string]int{}
	show := map[string]string{}
	for _, w := range words {
		if w.phrase != phrase {
			continue
		}
		n := norm(w.text, spell)
		count[n]++
		if _, ok := show[n]; !ok {
			show[n] = w.text
		}
	}
	for j := 1; j < len(got); j++ {
		a := norm(got[j-1].W, spell)
		if a == "" || a != norm(got[j].W, spell) || count[a] != 1 {
			continue
		}
		if (owner[j-1] == phrase && owner[j] < 0) || (owner[j] == phrase && owner[j-1] < 0) {
			return show[a]
		}
	}
	return ""
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
