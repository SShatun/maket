import json
import sys

import mlx.core as mx
import mlx_whisper

job = json.load(open(sys.argv[1], encoding='utf-8'))
mx.set_memory_limit(6 * 1024 ** 3)
for item in job['items']:
    r = mlx_whisper.transcribe(item['wav'], path_or_hf_repo=job['model'], language='ru', word_timestamps=True,
                               verbose=False, condition_on_previous_text=False)
    words = [{'w': w['word'].strip(), 's': w['start'], 'e': w['end']} for seg in r['segments'] for w in seg['words']]
    json.dump(words, open(item['out'], 'w', encoding='utf-8'), ensure_ascii=False)
