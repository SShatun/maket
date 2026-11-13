# Установка maket с голосом

Нужен Mac с Apple Silicon, Go 1.25+, ffmpeg, Python 3 и `uv`. Проверка перед установкой назовёт недостающие программы и свободное место. Python 3.12.12 и 3.11.14 для голосовых окружений `uv` ставит сам. Устанавливать из корня репозитория:

```bash
bash tools/install.sh
```

По умолчанию всё лежит в `~/.local/share/maket`. Другое место: `bash tools/install.sh --prefix /путь/к/каталогу`. Повторный запуск продолжает прерванную загрузку и пропускает готовые модели. При ошибке скрипт печатает этап и команду для повторного запуска. Нужно не меньше 24 ГиБ свободного места; проверенная установка заняла 17 ГиБ. Время зависит от сети и Mac: ориентир 15–60 минут.

Проверка без сетевых загрузок прогоняет фразу через RUAccent, Qwen TTS, Whisper и проверку тембра ECAPA/Praat:

```bash
bash tools/install.sh --check
```

Первый ролик с голосом:

```bash
go run ./cmd/maket render scenes/rts-retry.json -o out/first.mp4 -workers 4
```

Для ролика без голоса достаточно Go и ffmpeg, установка моделей не нужна:

```bash
go run ./cmd/maket render scenes/rts-retry.json -o out/silent.mp4 -silent -workers 4
```

`-silent` убирает речь, но звуковые эффекты стиля остаются. Для полностью немого файла:

```bash
ffmpeg -i out/silent.mp4 -c:v copy -an out/no-audio.mp4
```

`maket.json` содержит общие настройки. Установщик создаёт `maket.local.json` с абсолютными путями к личной установке; файл игнорируется Git и перекрывает только указанные в нём поля. Перед переносом старой рабочей папки владельцу нужно сохранить прежнее содержимое `maket.json` в `maket.local.json` в своём checkout.

Модель Qwen с ударениями взята из публичного `siriusfreak/qwen3-tts-12hz-1.7b-ru-stress-cf`, ревизия `c16b3af3db3095120efc1ebb73f6522442bcb3b1`, и локально переведена в MLX BF16 без квантования. RUAccent загружается из `ruaccent/accentuator`, Whisper из `mlx-community/whisper-large-v3-mlx`, ECAPA из `speechbrain/spkrec-ecapa-voxceleb`. Для всех моделей в скрипте закреплены ревизии, совпавшие с рабочими файлами владельца. Версии библиотек каждого окружения зафиксированы в `tools/voice/requirements/` по работающим окружениям владельца.
