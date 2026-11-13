#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd -P)
prefix=${HOME}/.local/share/maket
mode=install
step=preflight

while (($#)); do
    case "$1" in
        --prefix)
            prefix=${2:?missing path after --prefix}
            shift 2
            ;;
        --check)
            mode=check
            shift
            ;;
        *)
            printf 'Unknown argument: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

trap 'printf "Failed at step: %s\nRerun: bash tools/install.sh --prefix %q\n" "$step" "$prefix" >&2' ERR

if [[ $mode == check ]]; then
    step=check
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    export MAKET_ROOT=$root MAKET_TMP=$tmp
    python3 - <<'PY'
import json
import os
import pathlib
import shlex
import subprocess
import sys

root = pathlib.Path(os.environ['MAKET_ROOT'])
tmp = pathlib.Path(os.environ['MAKET_TMP'])
config = json.loads((root / 'maket.json').read_text())
local = root / 'maket.local.json'
if local.exists():
    config.update(json.loads(local.read_text()))
fields = ['tts_python', 'stress_python', 'tts_model', 'ruaccent_dir', 'whisper_model', 'checks_python', 'checks_model']
for field in fields:
    value = config.get(field)
    if not value or not pathlib.Path(value).exists():
        sys.exit(f'{field}: missing {value!r}; run bash tools/install.sh')
models = {
    'tts_model': ['config.json', 'model.safetensors', 'speech_tokenizer/model.safetensors'],
    'ruaccent_dir': ['dictionary/accents.json.gz', 'nn/nn_accent/model.onnx', 'nn/nn_omograph/turbo3.1/model.onnx', 'nn/nn_yo_homograph_resolver/model.onnx'],
    'whisper_model': ['config.json', 'weights.npz'],
    'checks_model': ['hyperparams.yaml', 'embedding_model.ckpt'],
}
for field, files in models.items():
    for name in files:
        path = pathlib.Path(config[field]) / name
        if not path.is_file() or path.stat().st_size == 0:
            sys.exit(f'{field}: missing {path}; rerun bash tools/install.sh')
module = subprocess.check_output([config['stress_python'], '-c', 'import importlib.util; from pathlib import Path; print(Path(importlib.util.find_spec("ruaccent").origin).parent)'], text=True).strip()
package_model = pathlib.Path(module) / 'koziev/rupostagger/rupostagger.model'
if not package_model.is_file():
    sys.exit(f'RUAccent package model: missing {package_model}; rerun bash tools/install.sh')
for field, modules in [('tts_python', 'mlx_audio, mlx_whisper'), ('stress_python', 'ruaccent'), ('checks_python', 'speechbrain, parselmouth')]:
    for module in modules.split(', '):
        subprocess.run([config[field], '-c', f'import {module}'], check=True)
spec = json.loads((root / 'voices/dima/voice.json').read_text())
phrase = 'Привет, это проверка голоса.'
(tmp / 'stress.json').write_text(json.dumps({'phrases': [phrase], 'workdir': config['ruaccent_dir'], 'out': str(tmp / 'marked.json')}))
(tmp / 'tts.json').write_text(json.dumps({'model': config['tts_model'], 'seed': 42, 'ref_audio': str(root / 'voices/dima' / spec['ref_audio']), 'ref_text': spec['ref_text'], 'items': [{'text': phrase, 'out': str(tmp / 'sample.wav')}]}))
(tmp / 'asr.json').write_text(json.dumps({'model': config['whisper_model'], 'items': [{'wav': str(tmp / 'sample.wav'), 'out': str(tmp / 'words.json')}]}))
(tmp / 'checks.json').write_text(json.dumps({'model': config['checks_model'], 'ref': str(root / 'voices/dima' / spec['ref_audio']), 'wavs': [str(tmp / 'sample.wav')], 'out': str(tmp / 'checks-out.json')}))
with (tmp / 'paths.sh').open('w') as out:
    for field in ['tts_python', 'stress_python', 'checks_python']:
        out.write(field + '=' + shlex.quote(config[field]) + '\n')
print('Paths, model files and imports: OK', flush=True)
PY
    source "$tmp/paths.sh"
    export HF_HUB_OFFLINE=1
    "$stress_python" "$root/tools/voice/stress.py" "$tmp/stress.json"
    python3 - "$tmp" <<'PY'
import json
import pathlib
import sys
p = pathlib.Path(sys.argv[1])
marked = json.loads((p / 'marked.json').read_text())['marked'][0]
job = json.loads((p / 'tts.json').read_text())
job['items'][0]['text'] = marked
(p / 'tts.json').write_text(json.dumps(job, ensure_ascii=False))
print('Stress:', marked, flush=True)
PY
    busy=${MAKET_BUSY_CHECK:-}
    if [[ -n $busy ]]; then
        while bash "$busy"; do
            printf 'Voice machine busy; checking again in 30 seconds\n'
            sleep 30
        done
    fi
    "$tts_python" "$root/tools/voice/tts.py" "$tmp/tts.json"
    if [[ -n $busy ]]; then
        while bash "$busy"; do
            printf 'Voice machine busy; checking again in 30 seconds\n'
            sleep 30
        done
    fi
    "$tts_python" "$root/tools/voice/asr.py" "$tmp/asr.json"
    "$checks_python" "$root/tools/voice/checks.py" "$tmp/checks.json"
    python3 - "$tmp" <<'PY'
import json
import pathlib
import sys
p = pathlib.Path(sys.argv[1])
if (p / 'sample.wav').stat().st_size < 1000:
    sys.exit('TTS produced no usable WAV')
words = json.loads((p / 'words.json').read_text())
checks = json.loads((p / 'checks-out.json').read_text())
if not words or not checks or not 0 <= checks[0]['ref'] <= 1:
    sys.exit('Recognition or timbre check returned no result')
print('Recognition:', ' '.join(word['w'] for word in words))
print('Timbre similarity:', round(checks[0]['ref'], 3), 'end fall:', round(checks[0]['fall'], 3))
print('Installation check: OK')
PY
    exit 0
fi

missing=0
if [[ $(uname -s) != Darwin || $(uname -m) != arm64 ]]; then
    printf 'Need macOS on Apple Silicon (arm64).\n' >&2
    missing=1
fi
for command in go ffmpeg python3 uv; do
    if ! command -v "$command" >/dev/null 2>&1; then
        printf 'Missing %s. Install it before rerunning.\n' "$command" >&2
        missing=1
    fi
done
if ((missing)); then
    exit 1
fi
if ! go version | grep -Eq 'go1\.(2[5-9]|[3-9][0-9])'; then
    printf 'Need Go 1.25 or newer.\n' >&2
    exit 1
fi
mkdir -p "$prefix"
prefix=$(cd "$prefix" && pwd -P)
available_kb=$(df -Pk "$prefix" | awk 'END {print $4}')
if ((available_kb < 24 * 1024 * 1024)); then
    printf 'Need at least 24 GiB free at %s; available: %s GiB.\n' "$prefix" "$((available_kb / 1024 / 1024))" >&2
    exit 1
fi
printf 'Preflight: Apple Silicon, Go, ffmpeg, Python, uv and disk space OK (%s GiB free).\n' "$((available_kb / 1024 / 1024))"

export UV_PYTHON_INSTALL_DIR="$prefix/python" UV_CACHE_DIR="$prefix/uv-cache" HF_HOME="$prefix/hf-cache" HF_HUB_DISABLE_XET=1
step='Python 3.12.12 and 3.11.14'
uv python install 3.12.12 3.11.14

for entry in tts:3.12.12 stress:3.12.12 checks:3.11.14; do
    name=${entry%%:*}
    version=${entry#*:}
    step="$name environment"
    if [[ ! -x $prefix/$name/bin/python ]]; then
        uv venv --python "$version" "$prefix/$name"
    fi
    uv pip install --python "$prefix/$name/bin/python" --no-deps -r "$root/tools/voice/requirements/$name.txt"
done

step='stress model download'
if [[ ! -s $prefix/models/stress-source/model.safetensors || ! -s $prefix/models/stress-source/config.json || ! -s $prefix/models/stress-source/speech_tokenizer/model.safetensors ]]; then
    mkdir -p "$prefix/models/stress-source"
    "$prefix/tts/bin/python" - "$prefix/models/stress-source" <<'PY'
import sys
from huggingface_hub import snapshot_download
snapshot_download('siriusfreak/qwen3-tts-12hz-1.7b-ru-stress-cf', revision='c16b3af3db3095120efc1ebb73f6522442bcb3b1', local_dir=sys.argv[1], allow_patterns=['*.json', '*.txt', '*.safetensors', 'README.md'], ignore_patterns=['lora/*', 'scripts/*', 'meter/*', 'eval/*'])
PY
fi

step='stress model BF16 conversion'
if [[ ! -s $prefix/models/stress-model-full-bf16/model.safetensors ]]; then
    rm -rf "$prefix/models/stress-model-full-bf16.partial"
    "$prefix/tts/bin/python" - "$prefix/models/stress-source" "$prefix/models/stress-model-full-bf16.partial" <<'PY'
import sys
import mlx.core as mx
from mlx_audio.convert import convert
mx.set_memory_limit(12 * 1024 ** 3)
mx.set_cache_limit(256 * 1024 ** 2)
convert(sys.argv[1], sys.argv[2], quantize=False, dtype='bfloat16', model_domain='tts')
PY
    mv "$prefix/models/stress-model-full-bf16.partial" "$prefix/models/stress-model-full-bf16"
fi

step='RUAccent model download'
if [[ ! -s $prefix/models/ruaccent/nn/nn_omograph/turbo3.1/model.onnx || ! -s $prefix/models/ruaccent/nn/nn_accent/model.onnx || ! -s $prefix/models/ruaccent/nn/nn_yo_homograph_resolver/model.onnx || ! -s $prefix/models/ruaccent/dictionary/accents.json.gz ]]; then
    "$prefix/stress/bin/python" - "$prefix/models/ruaccent" <<'PY'
import sys
from huggingface_hub import snapshot_download
snapshot_download('ruaccent/accentuator', revision='b78ae5ea1e62beaf138bed1865cd8c3b0b5ca855', local_dir=sys.argv[1], allow_patterns=['dictionary/**', 'nn/nn_accent/**', 'nn/nn_omograph/turbo3.1/**', 'nn/nn_yo_homograph_resolver/**', 'nn/nn_stress_usage_predictor/**'])
PY
fi

step='RUAccent package models'
"$prefix/stress/bin/python" - <<'PY'
import importlib.util
from pathlib import Path
from huggingface_hub import snapshot_download
module = Path(importlib.util.find_spec('ruaccent').origin).parent
if not (module / 'koziev/rupostagger/rupostagger.model').is_file():
    snapshot_download('ruaccent/accentuator', revision='b78ae5ea1e62beaf138bed1865cd8c3b0b5ca855', local_dir=module, allow_patterns=['koziev/**'])
PY

step='Whisper model download'
if [[ ! -s $prefix/models/whisper-large-v3-mlx/weights.npz || ! -s $prefix/models/whisper-large-v3-mlx/config.json ]]; then
    "$prefix/tts/bin/python" - "$prefix/models/whisper-large-v3-mlx" <<'PY'
import sys
from huggingface_hub import snapshot_download
snapshot_download('mlx-community/whisper-large-v3-mlx', revision='49e6aa286ad60c14352c404340ded53710378a11', local_dir=sys.argv[1])
PY
fi

step='ECAPA model download'
if [[ ! -s $prefix/models/ecapa/embedding_model.ckpt || ! -s $prefix/models/ecapa/hyperparams.yaml ]]; then
    "$prefix/checks/bin/python" - "$prefix/models/ecapa" <<'PY'
import sys
from huggingface_hub import snapshot_download
snapshot_download('speechbrain/spkrec-ecapa-voxceleb', revision='0f99f2d0ebe89ac095bcc5903c4dd8f72b367286', local_dir=sys.argv[1])
PY
fi

step='local settings'
MAKET_PREFIX=$prefix MAKET_ROOT=$root python3 - <<'PY'
import json
import os
from pathlib import Path
prefix = Path(os.environ['MAKET_PREFIX'])
root = Path(os.environ['MAKET_ROOT'])
config = {
    'tts_python': str(prefix / 'tts/bin/python'),
    'stress_python': str(prefix / 'stress/bin/python'),
    'tts_model': str(prefix / 'models/stress-model-full-bf16'),
    'ruaccent_dir': str(prefix / 'models/ruaccent'),
    'whisper_model': str(prefix / 'models/whisper-large-v3-mlx'),
    'checks_python': str(prefix / 'checks/bin/python'),
    'checks_model': str(prefix / 'models/ecapa'),
}
target = root / 'maket.local.json'
temporary = root / 'maket.local.json.tmp'
temporary.write_text(json.dumps(config, ensure_ascii=False, indent=2) + '\n')
temporary.replace(target)
print('Wrote', target)
PY

step='Go build'
mkdir -p "$prefix/bin"
(cd "$root" && go build -o "$prefix/bin/maket" ./cmd/maket)
printf 'Install complete. Check: bash tools/install.sh --check\n'
