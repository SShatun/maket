import json, re, sys, unicodedata
from pathlib import Path
from ruaccent import RUAccent

bench = Path('/Users/georgy.kulikov/qwen-tts-benchmark')
out = Path(sys.argv[1])
phrases = [
    'сессия упала?',
    'ты пишешь: ну чё там? ответа нет.',
    'пишешь: пиши. опять тишина.',
    'но сессия жива. она просто не читает очередь.',
    'она ждёт ответа в окошке, которого ты не видишь.',
    'окошко спрашивает: можно войти в копию ветки?',
    'жмёшь йес, и сообщения пошли.',
    'она не упала. она ждёт окошко.',
]
a = RUAccent()
a.load(omograph_model_size='turbo3.1', use_dictionary=True, device='CPU',
       workdir=str(bench / 'full_stress_test/ruaccent_models'),
       custom_dict={'йес': 'й+ес', 'пошли': 'пошл+и', 'чё': 'ч+ё'})
rows = []
for p in phrases:
    raw = a.process_all(p)
    marked = unicodedata.normalize('NFC', re.sub(r'\+([а-яё])', lambda m: m.group(1) + '́', raw))
    assert marked.replace('́', '') == p, (p, marked)
    rows.append({'plain': p, 'raw': raw, 'marked': marked})
    print(marked)
(out / 'phrases.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2))
