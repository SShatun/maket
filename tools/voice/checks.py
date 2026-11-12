import json
import sys
import warnings

warnings.filterwarnings('ignore')

import numpy as np
import parselmouth
import soundfile as sf
import torch
from scipy.signal import resample_poly
from speechbrain.inference.speaker import EncoderClassifier

job = json.load(open(sys.argv[1], encoding='utf-8'))
torch.set_num_threads(4)
model = EncoderClassifier.from_hparams(source=job['model'], savedir=job['model'],
                                       overrides={'pretrained_path': job['model']}, run_opts={'device': 'cpu'})


def read(path):
    wave, rate = sf.read(path, dtype='float32', always_2d=True)
    return wave.mean(axis=1), rate


def embed(path):
    wave, rate = read(path)
    if rate != 16000:
        g = np.gcd(rate, 16000)
        wave = resample_poly(wave, 16000 // g, rate // g).astype(np.float32)
    with torch.inference_mode():
        e = model.encode_batch(torch.from_numpy(wave).unsqueeze(0)).flatten().numpy()
    return e / np.linalg.norm(e)


def fall(path):
    wave, rate = read(path)
    pitch = parselmouth.Sound(wave, rate).to_pitch_ac(time_step=0.01, pitch_floor=60, pitch_ceiling=400)
    t, f = pitch.xs(), pitch.selected_array['frequency']
    voiced = t[f > 0]
    if len(voiced) < 5:
        return 0.0
    m = (t >= voiced[-1] - 0.4) & (f > 0)
    if m.sum() < 5:
        return 0.0
    st = 12 * np.log2(f[m] / np.median(f[m]))
    return float(np.polyfit(t[m], st, 1)[0] * 0.4)


ref = embed(job['ref'])
embs = [embed(w) for w in job['wavs']]
out = []
for i, (w, e) in enumerate(zip(job['wavs'], embs)):
    out.append({
        'ref': float(e @ ref),
        'prev': float(e @ embs[i - 1]) if i > 0 else None,
        'next': float(e @ embs[i + 1]) if i + 1 < len(embs) else None,
        'fall': fall(w),
    })
json.dump(out, open(job['out'], 'w', encoding='utf-8'))
