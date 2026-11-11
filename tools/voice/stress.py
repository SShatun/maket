import json
import re
import sys
import unicodedata

from ruaccent import RUAccent

job = json.load(open(sys.argv[1], encoding='utf-8'))
acc = RUAccent()
acc.load(omograph_model_size='turbo3.1', use_dictionary=True, device='CPU', workdir=job['workdir'],
         custom_dict=job.get('custom_dict') or {})


def accent(text):
    try:
        return acc.process_all(text)
    except ValueError:
        out = []
        for word in text.split():
            try:
                out.append(acc.process_all(word))
            except ValueError:
                out.append(word)
        return ' '.join(out)


marked = []
for text in job['phrases']:
    raw = accent(text)
    marked.append(unicodedata.normalize('NFC', re.sub(r'\+([а-яёА-ЯЁ])', lambda m: m.group(1) + '́', raw)))
json.dump({'marked': marked}, open(job['out'], 'w', encoding='utf-8'), ensure_ascii=False)
