import json
import os
import signal
import subprocess
import sys
import threading
import time

signal.alarm(900)
job = json.load(open(sys.argv[1], encoding='utf-8'))
stop = threading.Event()


def pressure():
    return int(subprocess.check_output(['sysctl', '-n', 'kern.memorystatus_vm_pressure_level'], text=True))


def monitor():
    since = None
    while not stop.is_set():
        since = (since or time.monotonic()) if pressure() >= 4 else None
        if since is not None and time.monotonic() - since >= 6:
            print('SAFETY_ABORT: memory pressure', file=sys.stderr, flush=True)
            os._exit(70)
        if stop.wait(2):
            break


threading.Thread(target=monitor, daemon=True).start()
import mlx.core as mx
from mlx_audio.audio_io import write
from mlx_audio.tts.utils import load_model

mx.set_memory_limit(12 * 1024 ** 3)
mx.set_cache_limit(256 * 1024 ** 2)
model = load_model(job['model'])
for item in job['items']:
    mx.random.seed(job['seed'])
    res = list(model.generate(text=item['text'], lang_code='Russian', ref_audio=job['ref_audio'], ref_text=job['ref_text'],
                              temperature=0.9, top_p=1.0, top_k=50, repetition_penalty=1.05, max_tokens=1200,
                              split_pattern=None, stream=False, verbose=False))
    audio = mx.concatenate([r.audio for r in res])
    mx.eval(audio)
    write(item['out'], audio, res[0].sample_rate)
stop.set()
