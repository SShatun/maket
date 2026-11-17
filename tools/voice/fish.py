import json
import struct
import subprocess
import sys
import urllib.error
import urllib.request
import wave
from pathlib import Path


class SynthesisError(Exception):
    pass


def pack(value):
    if isinstance(value, bool):
        return b'\xc3' if value else b'\xc2'
    if isinstance(value, int):
        return b'\xd2' + struct.pack('>i', value)
    if isinstance(value, str):
        data = value.encode('utf-8')
        return b'\xdb' + struct.pack('>I', len(data)) + data
    if isinstance(value, bytes):
        return b'\xc6' + struct.pack('>I', len(value)) + value
    if isinstance(value, list):
        return b'\xdc' + struct.pack('>H', len(value)) + b''.join(pack(v) for v in value)
    if isinstance(value, dict):
        return b'\xde' + struct.pack('>H', len(value)) + b''.join(pack(k) + pack(v) for k, v in value.items())
    raise TypeError(type(value).__name__)


def main():
    job = json.loads(Path(sys.argv[1]).read_text())
    directory = Path(job['directory'])
    key = None
    reference = None
    for i, text in enumerate(job['texts'], 1):
        wav = directory / f'phrase-{i}.wav'
        if wav.is_file() and wav.stat().st_size > 44:
            continue
        if key is None:
            key = Path(job['key_file']).read_text().strip()
            if not key or '\n' in key or '\r' in key:
                raise SynthesisError('fish: key file must contain one nonempty API key')
            reference = Path(job['ref_audio']).read_bytes()
        request = {'text': text, 'references': [{'audio': reference, 'text': job['ref_text']}],
                   'format': 'wav', 'sample_rate': 44100, 'normalize': True, 'latency': 'normal'}
        req = urllib.request.Request('https://api.fish.audio/v1/tts', data=pack(request),
                                     headers={'Authorization': 'Bearer ' + key, 'model': job['model'],
                                              'Content-Type': 'application/msgpack'})
        try:
            with urllib.request.urlopen(req, timeout=180) as response:
                data = response.read()
        except urllib.error.HTTPError as error:
            raise SynthesisError(f'fish: phrase {i}: HTTP {error.code}; request not retried') from None
        except (urllib.error.URLError, TimeoutError):
            raise SynthesisError(f'fish: phrase {i}: connection failed; request not retried to avoid duplicate billing') from None
        if data[:4] != b'RIFF' or data[8:12] != b'WAVE':
            raise SynthesisError(f'fish: phrase {i}: invalid WAV; response not cached')
        temporary = wav.with_suffix('.partial.wav')
        try:
            result = subprocess.run(['ffmpeg', '-v', 'error', '-y', '-i', 'pipe:0', '-ac', '1',
                                     '-ar', str(job['rate']), '-c:a', 'pcm_s16le', str(temporary)],
                                    input=data, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
            if result.returncode:
                raise SynthesisError(f'fish: phrase {i}: ffmpeg conversion failed; response not cached')
            with wave.open(str(temporary), 'rb') as audio:
                if audio.getnframes() == 0 or audio.getnchannels() != 1 or audio.getsampwidth() != 2 or audio.getframerate() != job['rate']:
                    raise SynthesisError(f'fish: phrase {i}: empty or invalid audio; response not cached')
            temporary.replace(wav)
        finally:
            if temporary.exists():
                temporary.unlink()


try:
    main()
except SynthesisError as error:
    print(str(error), file=sys.stderr)
    sys.exit(1)
except Exception as error:
    print(f'fish: synthesis failed ({type(error).__name__}); request not retried', file=sys.stderr)
    sys.exit(1)
