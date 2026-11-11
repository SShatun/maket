import json, sys
from pathlib import Path
import mlx.core as mx
import mlx_whisper

out = Path(sys.argv[1])
mx.set_memory_limit(6 * 1024 ** 3)
r = mlx_whisper.transcribe(str(out / 'narration.wav'),
                           path_or_hf_repo='/Users/georgy.kulikov/.codex/skills/obs-transcribe/models/whisper-large-v3-mlx',
                           language='ru', word_timestamps=True, verbose=False, condition_on_previous_text=False)
words = [{'w': w['word'].strip(), 's': round(w['start'], 2), 'e': round(w['end'], 2)} for seg in r['segments'] for w in seg['words']]
(out / 'words.json').write_text(json.dumps(words, ensure_ascii=False, indent=1))
print(r['text'])
for w in words:
    print(w['s'], w['e'], w['w'])
