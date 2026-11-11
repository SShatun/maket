import json, re, sys, unicodedata
from pathlib import Path
from ruaccent import RUAccent

bench = Path('/Users/georgy.kulikov/qwen-tts-benchmark')
out = Path(sys.argv[1])
phrases = json.loads(open(out / 'plain.json').read())
a = RUAccent()
a.load(omograph_model_size='turbo3.1', use_dictionary=True, device='CPU',
       workdir=str(bench / 'full_stress_test/ruaccent_models'),
       custom_dict={'ресинк': 'рес+инк', 'ресинка': 'рес+инка', 'ноль': 'н+оль', 'пустой': 'пуст+ой'})
rows = []
for p in phrases:
    raw = a.process_all(p)
    marked = unicodedata.normalize('NFC', re.sub(r'\+([а-яё])', lambda m: m.group(1) + '́', raw))
    assert marked.replace('́', '') == p, (p, marked)
    rows.append({'plain': p, 'raw': raw, 'marked': marked})
    print(marked)
(out / 'phrases.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2))
