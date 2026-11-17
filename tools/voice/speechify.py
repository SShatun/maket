import base64
import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path


def pronunciation(word, marked):
    if marked.startswith('ipa:'):
        return {'text': word, 'ipa': marked[4:].strip()}
    plain = marked.replace('\u0301', '')
    if '\u0301' not in marked:
        return {'text': word, 'alias': plain} if plain.lower() != word.lower() else None
    vowels = 'аеёиоуыэюя'
    expected = sum(c.lower() in vowels for c in marked[:marked.index('\u0301')])
    try:
        ipa = subprocess.check_output(['espeak-ng', '-q', '-v', 'ru', '--ipa', '--sep=|', plain], text=True).strip()
    except FileNotFoundError:
        raise ValueError('speechify: dictionary stress needs espeak-ng; run bash tools/install.sh --speechify') from None
    phones = [p for p in ipa.replace('\n', '|').split('|') if p.strip()]
    found, count, output = 0, 0, []
    for phone in phones:
        phone = phone.strip().replace('͡', '').replace('g', 'ɡ')
        stressed = 'ˈ' in phone
        phone = phone.replace('ˈ', '').replace('ˌ', '')
        if phone[0] in 'aeiouyæøœɐɑɒɔɘəɚɛɜɝɞɤɨɪɯɵɶʉʊʌʏᵻ':
            count += 1
            if stressed:
                found = count
        if phone.endswith('ʲ') and phone not in {'bʲ','dʲ','kʲ','mʲ','nʲ','pʲ','rʲ','sʲ','tʲ','vʲ','ɡʲ'}:
            output.extend([phone[:-1], 'ʲ'])
        else:
            output.append(('ˈ' if stressed else '') + phone)
    if count != sum(c.lower() in vowels for c in plain) or found != expected:
        raise ValueError(f'speechify: cannot safely convert {marked!r} to IPA; set stress[{word.lower()!r}] to "ipa:..."')
    return {'text': word, 'ipa': ' '.join(output)}


def main():
    job = json.loads(Path(sys.argv[1]).read_text())
    request = {'model': 'simba-3.5', 'voice_id': job['voice'], 'language': 'ru-RU',
               'audio_format': 'wav', 'input': ' '.join(job['texts'])}
    if len(request['input']) > 2000:
        raise ValueError('speechify: one continuous take supports at most 2000 input characters')
    pronunciations = {}
    for word, marked in job['overrides']:
        entry = pronunciation(word, marked)
        if entry is None:
            continue
        key = word.lower()
        entry['text'] = key
        if key in pronunciations and pronunciations[key] != entry:
            raise ValueError(f'speechify: conflicting pronunciations for {word!r} in one take')
        pronunciations[key] = entry
    if pronunciations:
        request['pronunciations'] = list(pronunciations.values())
    directory = Path(job['directory'])
    request_path = directory / 'request.json'
    response_path = directory / 'response.json'
    wav = directory / 'source.wav'
    if response_path.is_file() and wav.is_file() and wav.stat().st_size > 0:
        return
    key = Path(job['key_file']).read_text().strip()
    if not key:
        raise ValueError('speechify: empty key file')
    req = urllib.request.Request('https://api.speechify.ai/v1/audio/speech',
                                 data=json.dumps(request).encode(),
                                 headers={'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req, timeout=180) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f'speechify: HTTP {error.code}; request not retried') from None
    except (urllib.error.URLError, TimeoutError):
        raise RuntimeError('speechify: connection failed; request not retried to avoid duplicate billing') from None
    data = base64.b64decode(result.pop('audio_data', '') or '', validate=True)
    if not data:
        raise RuntimeError('speechify: empty audio_data; response not cached, rerun to request audio again')
    request_path.write_text(json.dumps(request, ensure_ascii=False, indent=2))
    temporary = wav.with_suffix('.partial.wav')
    temporary.write_bytes(data)
    temporary.replace(wav)
    temporary = response_path.with_suffix('.partial.json')
    temporary.write_text(json.dumps(result, ensure_ascii=False, indent=2))
    temporary.replace(response_path)


try:
    main()
except Exception as error:
    print(str(error), file=sys.stderr)
    sys.exit(1)
