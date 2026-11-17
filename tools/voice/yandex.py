import base64
import json
import sys
import urllib.error
import urllib.request
import wave
from pathlib import Path


class SynthesisError(Exception):
    pass


def main():
    job = json.loads(Path(sys.argv[1]).read_text())
    directory = Path(job['directory'])
    key = None
    for i, text in enumerate(job['texts'], 1):
        wav = directory / f'phrase-{i}.wav'
        if wav.is_file() and wav.stat().st_size > 44:
            continue
        if key is None:
            key = Path(job['key_file']).read_text().strip()
            if not key or '\n' in key or '\r' in key:
                raise SynthesisError('yandex: key file must contain one nonempty API key')
        request = {'text': text, 'hints': [{'voice': job['voice']}, {'role': job['role']}],
                   'outputAudioSpec': {'rawAudio': {'audioEncoding': 'LINEAR16_PCM',
                                                   'sampleRateHertz': str(job['rate'])}}}
        req = urllib.request.Request('https://tts.api.cloud.yandex.net/tts/v3/utteranceSynthesis',
                                     data=json.dumps(request).encode(),
                                     headers={'Authorization': 'Api-Key ' + key, 'Content-Type': 'application/json'})
        data = bytearray()
        try:
            with urllib.request.urlopen(req, timeout=180) as response:
                for line in response:
                    if not line.strip():
                        continue
                    result = json.loads(line)
                    if 'error' in result:
                        raise SynthesisError(f'yandex: phrase {i}: API error; response not cached, request not retried')
                    chunk = result.get('result', {}).get('audioChunk', {}).get('data', '')
                    data.extend(base64.b64decode(chunk, validate=True))
        except urllib.error.HTTPError as error:
            raise SynthesisError(f'yandex: phrase {i}: HTTP {error.code}; request not retried') from None
        except (urllib.error.URLError, TimeoutError):
            raise SynthesisError(f'yandex: phrase {i}: connection failed; request not retried to avoid duplicate billing') from None
        if not data or len(data) % 2:
            raise SynthesisError(f'yandex: phrase {i}: empty or invalid PCM; response not cached')
        temporary = wav.with_suffix('.partial.wav')
        with wave.open(str(temporary), 'wb') as output:
            output.setnchannels(1)
            output.setsampwidth(2)
            output.setframerate(job['rate'])
            output.writeframes(data)
        temporary.replace(wav)


try:
    main()
except SynthesisError as error:
    print(str(error), file=sys.stderr)
    sys.exit(1)
except Exception as error:
    print(f'yandex: synthesis failed ({type(error).__name__}); request not retried', file=sys.stderr)
    sys.exit(1)
