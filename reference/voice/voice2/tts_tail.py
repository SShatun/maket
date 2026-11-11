import json, re, signal, subprocess, sys, threading, time, os
from pathlib import Path

signal.alarm(600)
bench = Path('/Users/georgy.kulikov/qwen-tts-benchmark')
out = Path(sys.argv[1])
stop = threading.Event()


def pressure():
    return int(subprocess.check_output(['sysctl', '-n', 'kern.memorystatus_vm_pressure_level'], text=True))


def monitor():
    since = None
    while not stop.is_set():
        since = (since or time.monotonic()) if pressure() >= 4 else None
        if since is not None and time.monotonic() - since >= 6:
            print('SAFETY_ABORT', flush=True)
            os._exit(70)
        if stop.wait(2):
            break


threading.Thread(target=monitor, daemon=True).start()
import mlx.core as mx
from mlx_audio.tts.utils import load_model
from mlx_audio.audio_io import write

mx.set_memory_limit(12 * 1024 ** 3)
mx.set_cache_limit(256 * 1024 ** 2)
model = load_model(str(bench / 'stress-model-full-bf16'))
ref = str(bench / 'dialogs_five/masha_sveta_part2_359.wav')
reftext = [s for s in json.loads((bench / 'dialogs_five/sources.json').read_text())['samples'] if s['file'] == 'masha_sveta_part2_359.wav'][0]['text']
phrases = [r['marked'].replace('пуши́т', 'пу́шит').replace('сло́ва пушу́', 'слова́ пушу́').replace('про́ пу́ш', 'про пу́ш') for r in json.loads((out / 'phrases.json').read_text())]
text = phrases[-1]
mx.random.seed(42)
t = time.monotonic()
res = list(model.generate(text=text, lang_code='Russian', ref_audio=ref, ref_text=reftext, temperature=0.9,
                          top_p=1.0, top_k=50, repetition_penalty=1.05, max_tokens=1200, split_pattern=None,
                          stream=False, verbose=False))
audio = mx.concatenate([r.audio for r in res])
mx.eval(audio)
sr = res[0].sample_rate
write(str(out / 'tail.wav'), audio, sr)
print(json.dumps({'text': text, 'seconds': len(audio) / sr, 'gen': time.monotonic() - t,
                  'peak_gib': mx.get_peak_memory() / 1024 ** 3}, ensure_ascii=False), flush=True)
stop.set()
