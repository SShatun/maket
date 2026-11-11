# maket

Движок вертикальных тех-объяснялок в стиле «Макет»: JSON-сцена → mp4 1080×1920, 30 fps, со звуком.

## Установка

```
pip install pillow numpy
# ffmpeg должен быть в PATH (brew install ffmpeg)
```

## Запуск

```
python -m maket render scenes/buffered-channel.json out/buffered-channel.mp4
python -m maket render scenes/cache-aside.json out/cache-aside.mp4 --no-sfx
python -m maket stills scenes/cache-aside.json out/stills 2 6 12.5
```

Рендер 20-секундной сцены занимает около 20 секунд на одном ядре.

## Что где

- `maket/tokens.py` — все цвета, кегли, размеры, тайминги, громкости. Стиль меняется только здесь.
- `maket/engine.py` — сцена, изометрия, автокадрирование, отрисовка кадра.
- `maket/sfx.py` — звук, синтезируется из событий сцены.
- `scenes/` — примеры сцен.
- `maket/fonts/` — Inter и JetBrains Mono (SIL OFL, лицензии рядом).

Формат сцены, правила раскладки, движения и звука описаны в дизайн-системе «Макет» (разделы «Формат сцены» и «Как делать новый ролик»).
