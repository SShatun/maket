import json, signal, sys
from pathlib import Path

signal.alarm(600)
import mlx.core as mx
from mlx_audio.tts.utils import load_model
from mlx_audio.audio_io import write

bench = Path('/Users/georgy.kulikov/qwen-tts-benchmark')
out = Path(sys.argv[1])
mx.set_memory_limit(8 * 1024 ** 3)
mx.set_cache_limit(512 * 1024 ** 2)
model = load_model(str(bench / 'voice-design-model'))
text = 'Сегодня разберём, почему расчёт пропускает товар, хотя ресинк отработал без единой ошибки.'
variants = [
    ('baritone', 'Мужчина лет сорока, носитель русского языка, литературное произношение. Низкий тёплый баритон, '
                 'спокойная уверенная подача, как у ведущего научно-популярного ролика. Ровный темп, чёткая дикция, '
                 'смысловое ударение в конце фразы.'),
    ('engineer', 'Мужчина около тридцати пяти лет, носитель русского языка. Глубокий бархатный голос ниже среднего, '
                 'собранный и дружелюбный, как у опытного инженера, который объясняет коллеге баг. Неторопливо, '
                 'отчётливо, без театральности.'),
]
rows = []
for name, instruct in variants:
    mx.random.seed(7)
    res = list(model.generate_voice_design(text=text, language='Russian', instruct=instruct, temperature=0.9,
                                           top_p=1.0, top_k=50, repetition_penalty=1.05, max_tokens=300,
                                           verbose=False))
    audio = mx.concatenate([r.audio for r in res])
    mx.eval(audio)
    sr = res[0].sample_rate
    write(str(out / f'{name}.wav'), audio, sr)
    rows.append({'name': name, 'text': text, 'instruct': instruct, 'seconds': len(audio) / sr})
    print(json.dumps(rows[-1], ensure_ascii=False), flush=True)
(out / 'design.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2))
