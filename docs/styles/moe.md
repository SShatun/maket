# Моэ

## Что есть

- Вид: аниме-чиби со сглаживанием. Те же 70 образов, нарисованные девушками, зверьками и предметами с лицами: кэш — лисичка с рюкзаком, база — библиотекарь у картотеки, посылка — коробка с кошачьими ушками.
- Эмоции: у всех спокойствие, работа, поломка, ожидание, радость и грусть. У кошкодевочки `session` вдобавок «Бака!», цундере, смущение, шок, любовь, самодовольство.
- Фоны-настроения со своим светом на персонажах: закат, ночь, дождь, сакура, вспышка, озарение, тревога, смущение. Фон можно сменить посреди кадра.
- В кадре один–три персонажа крупно, каждый отдельно. Толпы нет.
- Действия: ходьба с подскоком, передача ценника с надписью от персонажа к персонажу, удар-шок со вспышкой, зумом и тряской, переход через белое или чёрное, титул эпизода.
- Голограммы — светящиеся панели строк: заголовок, строки с тонами (жёлтый, розовый, зелёный, белый), мигающая строка, крупный кегль. Ими показывают числа, таблицы, код и счётчики, которые меняются во времени.
- Схема из узлов и стрелок `diagram` поверх кадров.
- Звук по фону: аккорды, музыкальная шкатулка на титуле, бипы голограмм, удар на шоке, мяу и мурчание.
- Нет: толпы, стрелок между персонажами, таблички под ногами.

Аниме-чиби. Настроение задают фон и эмоции персонажей, технические детали показываются голограммами. Выбор стиля: `"style": "moe"`. `scenes/moe-tariff-cache.json` показывает возможности рисования, но не задаёт правило выбора мира.

## Кадры

Ролик строится из `shots` с полями `at`, `place`, `cast`, `do` (формат в `docs/scene.md`, раздел «Последовательность кадров»). Каждый кадр длится до начала следующего. Время событий абсолютное: секунды, `p2`, `p2:слово`, `p2.end`; допустимы смещения вроде `p2+0.4`.

Новые места добавляются по `docs/characters.md`: имя, подпись и синонимы в `look.json`, свет и `bright` в `backdrops/backgrounds.json`. Светлому фону нужен `bright: 1`, чтобы текст голограммы оставался на тёмной подложке.

`place` выбирает настроение: `sunset` для начала, `night` для ожидания, `rain` для потери, `sakura` для успеха, `flash` для шока, `idea` для озарения, `alert` для ошибки, `moe` для смущения. Свет персонажей подстраивается под фон.

- Два персонажа рядом: `size` около 0.85; крупный план: до 1.3.
- `at: [x, y]` у персонажа задаёт точку опоры в пикселях. Для заката и сакуры подходит y=1640, для ночи y=1600, для дождя y=1700.
- Камера плавно наезжает от 1 до 1.05 за кадр. Оставляйте место у краёв для наезда и тряски.
- Подпись располагается на y=1760, с обводкой и подсветкой произнесённых слов; она неподвижна относительно экрана.

## Персонажи

Каталог: `go run ./cmd/maket char list -look moe`. Таблицу ниже обновляет `char add`; подписи и синонимы помогают найти уже нарисованное.

<!-- characters:start -->
| kind | кто | синонимы | состояния | происхождение |
|---|---|---|---|---|
| `alert` | алерт | alert, алерт | broken, happy, idle, sad, wait, work | дизайнер |
| `avail` | доступность товара | avail, доступность товара | broken, happy, idle, sad, wait, work | дизайнер |
| `baby` | малыш | baby, малыш | broken, happy, idle, sad, wait, work | дизайнер |
| `balancer` | балансировщик | balancer, балансировщик | broken, happy, idle, sad, wait, work | дизайнер |
| `batch` | пакетная обработка | batch, пакетная обработка | broken, happy, idle, sad, wait, work | дизайнер |
| `bloom` | фильтр Блума | bloom, фильтр Блума | broken, happy, idle, sad, wait, work | дизайнер |
| `branch` | дриада с ростком в горшке | дриада, росток, горшок, ветка, копия ветки, git, branch | catch, flinch, grow, idle, pushed | дизайнер |
| `breaker` | предохранитель запросов | breaker, предохранитель запросов | broken, happy, idle, sad, wait, work | дизайнер |
| `btree` | B-дерево | btree, B-дерево | broken, happy, idle, sad, wait, work | дизайнер |
| `bucket` | бакет | bucket, бакет | broken, happy, idle, sad, wait, work | дизайнер |
| `budget` | бюджет | budget, бюджет | broken, happy, idle, sad, wait, work | дизайнер |
| `cache` | лисичка с рюкзаком | лисичка, рюкзак, дередере, кэш, готовые ответы, cache | catch, flinch, give, idle, miss | дизайнер |
| `card` | карточка товара | card, карточка товара | broken, happy, idle, sad, wait, work | дизайнер |
| `channel` | канал | channel, канал | broken, happy, idle, sad, wait, work | дизайнер |
| `client` | клиент | client, клиент | broken, happy, idle, sad, wait, work | дизайнер |
| `context` | контекст отмены | context, контекст отмены | broken, happy, idle, sad, wait, work | дизайнер |
| `courier` | курьер | courier, курьер | broken, happy, idle, sad, wait, work | дизайнер |
| `cpu` | процессор | cpu, процессор | broken, happy, idle, sad, wait, work | дизайнер |
| `cron` | планировщик | cron, планировщик | broken, happy, idle, sad, wait, work | дизайнер |
| `customs` | таможенник | customs, таможенник | broken, happy, idle, sad, wait, work | дизайнер |
| `db` | библиотекарь в очках у картотеки | библиотекарь, очки, картотека, база, хранилище, склад, db, database | catch, empty, flinch, give, idle | дизайнер |
| `deploy` | деплой | deploy, деплой | broken, happy, idle, sad, wait, work | дизайнер |
| `dialog` | девочка-зайка с табличками Yes/No | зайка, девочка с табличками, диалог, вопрос, подтверждение, dialog, prompt | catch, closed, flinch, idle, impatient | дизайнер |
| `disk` | диск | disk, диск | broken, happy, idle, sad, wait, work | дизайнер |
| `dlq` | очередь ошибок | dlq, очередь ошибок | broken, happy, idle, sad, wait, work | дизайнер |
| `docs` | документация | docs, документация | broken, happy, idle, sad, wait, work | дизайнер |
| `duty` | таможенная пошлина | duty, таможенная пошлина | broken, happy, idle, sad, wait, work | дизайнер |
| `err500` | ошибка 500 | err500, ошибка 500 | broken, happy, idle, sad, wait, work | дизайнер |
| `eta` | срок доставки | eta, срок доставки | broken, happy, idle, sad, wait, work | дизайнер |
| `external` | девушка в очках за стойкой | девушка за стойкой, очки, onee-san, внешний сервис, партнёр, приёмная, external, api | answer, catch, closed, flinch, idle | дизайнер |
| `flag` | флаг функции | flag, флаг функции | broken, happy, idle, sad, wait, work | дизайнер |
| `gc` | сборщик мусора | gc, сборщик мусора | broken, happy, idle, sad, wait, work | дизайнер |
| `gor` | горутина | gor, горутина | broken, happy, idle, sad, wait, work | дизайнер |
| `hash` | хеш | hash, хеш | broken, happy, idle, sad, wait, work | дизайнер |
| `heap` | куча | heap, куча | broken, happy, idle, sad, wait, work | дизайнер |
| `iface` | интерфейс | iface, интерфейс | broken, happy, idle, sad, wait, work | дизайнер |
| `leader` | лидер | leader, лидер | broken, happy, idle, sad, wait, work | дизайнер |
| `limiter` | ограничитель запросов | limiter, ограничитель запросов | broken, happy, idle, sad, wait, work | дизайнер |
| `list` | связный список | list, связный список | broken, happy, idle, sad, wait, work | дизайнер |
| `lock` | вахтёрша со списком и шлагбаумом | вахтёрша, список, шлагбаум, пропуск, проверка, доступ, lock, guard | allow, catch, check, deny, flinch, idle | дизайнер |
| `log` | лог | log, лог | broken, happy, idle, sad, wait, work | дизайнер |
| `mail` | почта | mail, почта | broken, happy, idle, sad, wait, work | дизайнер |
| `market` | маркетплейс | market, маркетплейс | broken, happy, idle, sad, wait, work | дизайнер |
| `metric` | метрика | metric, метрика | broken, happy, idle, sad, wait, work | дизайнер |
| `migration` | миграция | migration, миграция | broken, happy, idle, sad, wait, work | дизайнер |
| `mr` | запрос на слияние | mr, запрос на слияние | broken, happy, idle, sad, wait, work | дизайнер |
| `mutex` | мьютекс | mutex, мьютекс | broken, happy, idle, sad, wait, work | дизайнер |
| `oncall` | дежурный с рацией | oncall, дежурный с рацией | broken, happy, idle, sad, wait, work | дизайнер |
| `packet` | сетевой пакет | packet, сетевой пакет | broken, happy, idle, sad, wait, work | дизайнер |
| `painter` | художница с кистью и палитрой | художник, иллюстратор, рисование, artist, painter | catch, flinch, happy, idle, paint, wait | добавлен |
| `parcel` | картонная коробка с кошачьими ушками | коробка, кошачьи ушки, картон, посылка, товар, заказ, parcel, item | catch, flinch, happy, idle, sad, wait | дизайнер |
| `person` | человек с телефоном | человек, телефон, пользователь, зритель, клиент, person, user | catch, flinch, happy, idle, surprised, wait | дизайнер |
| `plane` | самолёт | plane, самолёт | broken, happy, idle, sad, wait, work | дизайнер |
| `pointer` | указатель | pointer, указатель | broken, happy, idle, sad, wait, work | дизайнер |
| `product` | продакт со свитком PRD | product, продакт со свитком PRD | broken, happy, idle, sad, wait, work | дизайнер |
| `pvz` | пункт выдачи | pvz, пункт выдачи | broken, happy, idle, sad, wait, work | дизайнер |
| `queue` | девочка в худи у конвейера | худи, конвейер, кудере, очередь, буфер, queue, buffer | catch, flinch, full, run, stop | дизайнер |
| `replica` | реплика | replica, реплика | broken, happy, idle, sad, wait, work | дизайнер |
| `resync` | пересинхронизация | resync, пересинхронизация | broken, happy, idle, sad, wait, work | дизайнер |
| `retry` | повтор запроса | retry, повтор запроса | broken, happy, idle, sad, wait, work | дизайнер |
| `select` | выбор канала | select, выбор канала | broken, happy, idle, sad, wait, work | дизайнер |
| `seller` | продавец | seller, продавец | broken, happy, idle, sad, wait, work | дизайнер |
| `semaphore` | семафор | semaphore, семафор | broken, happy, idle, sad, wait, work | дизайнер |
| `service` | горничная со счётами | горничная, счёты, сервис, расчёт, обработчик, service | bad, catch, count, flinch, idle, skip, wait | дизайнер |
| `session` | кошкодевочка с хвостиками и бубенчиком | кошкодевочка, сессия-тян, цундере, сессия, терминал, session | baka, catch, dere, flinch, happy, idle, love, sad, shock, sleep, smug, tsun, type, wait | дизайнер |
| `shard` | шард | shard, шард | broken, happy, idle, sad, wait, work | дизайнер |
| `shipping` | способ доставки | shipping, способ доставки | broken, happy, idle, sad, wait, work | дизайнер |
| `slice` | срез | slice, срез | broken, happy, idle, sad, wait, work | дизайнер |
| `stack` | стек | stack, стек | broken, happy, idle, sad, wait, work | дизайнер |
| `stock` | остаток товара | stock, остаток товара | broken, happy, idle, sad, wait, work | дизайнер |
| `struct` | структура | struct, структура | broken, happy, idle, sad, wait, work | дизайнер |
| `template` | девочка с папкой карточек | девочка с папкой, карточки, шаблон, настройки, способы доставки, template | catch, empty, flinch, full, idle | дизайнер |
| `ticket` | задача | ticket, задача | broken, happy, idle, sad, wait, work | дизайнер |
| `timeout` | таймаут | timeout, таймаут | broken, happy, idle, sad, wait, work | дизайнер |
| `topic` | почтальонка с сумкой | почтальонка, сумка, письма, почта, топик, сообщения, kafka, topic | catch, flinch, flow, idle, lag | дизайнер |
| `trace` | трассировка | trace, трассировка | broken, happy, idle, sad, wait, work | дизайнер |
| `truck` | грузовик | truck, грузовик | broken, happy, idle, sad, wait, work | дизайнер |
| `tx` | транзакция | tx, транзакция | broken, happy, idle, sad, wait, work | дизайнер |
| `waitgroup` | группа ожидания | waitgroup, группа ожидания | broken, happy, idle, sad, wait, work | дизайнер |
| `wal` | журнал записи | wal, журнал записи | broken, happy, idle, sad, wait, work | дизайнер |
| `warehouse` | склад | warehouse, склад | broken, happy, idle, sad, wait, work | дизайнер |
| `webhook` | вебхук | webhook, вебхук | broken, happy, idle, sad, wait, work | дизайнер |
| `whmap` | карта складов | whmap, карта складов | broken, happy, idle, sad, wait, work | дизайнер |
| `woman` | коллега с листом проверок | woman, коллега с листом проверок, женщина, коллега, продакт, QA | broken, happy, idle, sad, wait, work | дизайнер |
<!-- characters:end -->

```json
{"id": "CACHE", "kind": "cache", "role": "buffer", "at": [300, 1640],
 "size": 0.85, "state": "idle", "motion": "bob", "flip": false,
 "states": [["p2:свежую", "give"]]}
```

Роли: `external` (по умолчанию), `producer`, `consumer`, `buffer`, `storage`, `error`, `ok`. Они меняют цвет одежды и деталей. `flip` отражает персонажа вместе с нарисованными знаками. `motion` выбирает движение: `bob` для покачивания и дыхания, `jump` для прыжков, `none` для неподвижной фигуры. По умолчанию используется `bob`.

Состояния меняются через `states` или событие `state`. Разовые состояния держат последний кадр, `catch` и `flinch` возвращаются в спокойное состояние. Персонажи перечисляются по отдельности, без `crowd`.

## События

| `do` | Поля | Действие |
|---|---|---|
| `state` | `target`, `state`, `hold` | Начать состояние персонажа |
| `walk` | `target`, `to: [x, y]`, `dur` | Пройти в точку с подскоком шагами и разворотом по направлению движения; остаться там |
| `move` | `from`, `to`, `label`, `dur`, `arc`, `role`, `until`, `consume` | Передать ценник между персонажами или координатами; полёт по умолчанию 0.6 с, дуга 120 px |
| `bg` | `name`, `dur` | Сменить фон и свет; переход по умолчанию 0.4 с |
| `holo` | `id`, `rect`, `title`, `lines`, `role`, `size`, `blink`, `until` | Показать или обновить голограмму |
| `title` | `episode`, `lines`, `until` | Номер эпизода числом и одна-две короткие строки названия |
| `shock` | `zoom`, `shake` | Вспышка 0.12 с, ударный зум от 1.14, тряска 20 px с затуханием за 0.7 с |
| `cut` | `color`, `dur` | Переход через `black` или `white`; по умолчанию чёрный, 0.3 с |
| `sfx` | `name`, `dur` | Добавить звук |

Для склейки ставьте `cut` в начале нового кадра: `at` задаёт середину перехода. `title` и `holo` видны до `until` или конца кадра. `seed` задаёт воспроизводимую тряску.

```json
[
  {"at": 1, "do": "move", "from": "DB", "to": "CACHE", "label": "399 ₽", "role": "storage", "until": 4, "consume": true},
  {"at": 2, "do": "walk", "target": "CACHE", "to": [600, 1640], "dur": 1},
  {"at": 2, "do": "holo", "id": "timer", "rect": [90, 260, 490, 520], "size": 100, "lines": ["02"]},
  {"at": 3, "do": "holo", "id": "timer", "lines": ["01"]}
]
```

При заданном `until` ценник остаётся у руки получателя и движется вместе с ним. Без `until` он исчезает при прилёте; `consume: true` добавляет короткую вспышку при исчезновении. Вылет и прилёт сопровождаются `whoosh` и `land`. `size` у `holo` задаёт кегль строки, по умолчанию 25; при обновлении того же `id` размер сохраняется.

## Голограммы

```json
{"at": "p2", "do": "holo", "id": "tariff",
 "rect": [90, 260, 990, 520], "role": "buffer", "title": "КЭШ ТАРИФА",
 "lines": [["price: 349 ₽", "bad"], ["ttl: ещё не истёк", "hi"], "db: 399 ₽"],
 "blink": 1}
```

`rect` задаёт `[x0, y0, x1, y1]` внутри кадра. Для трёх строк подходит прямоугольник из примера. Панель появляется с мерцанием; на светлом фоне под текстом есть тёмное стекло.

Строка принимает текст, пару `[текст, тон]` или объект `{"text": "...", "tone": "accent", "appear": "p2+0.4"}`. Тоны `hi`/`accent` жёлтые, `bad`/`error` розовые, `ok` зелёные, `ink` белые; без тона используется цвет роли. Длинные строки лучше сокращать.

`blink` задаёт номер мигающей строки от нуля; `-1` отключает мигание. Повторный `id` в том же кадре обновляет панель: достаточно передать новые `lines`, остальные поля сохраняются. `lines: []` очищает строки; если мигающая строка удалена, передайте `blink: -1`.

## Звук и просмотр

Фоновый аккорд выбирается по фону. Титул сопровождается музыкальной шкатулкой, голограммы бипами, шок ударом. Радость, смущение, сон и выдача ответа тоже добавляют звуки.

Для `sfx` доступны `pad-sunset`, `pad-night`, `pad-idea`, `pad-sakura`, `musicbox`, `meow`, `heartbeat`, `purr`, `stab`, `blip`, `sparkle`, `whoosh`, `land`, `thud`, `buzz`, `pop`, `chime`. `dur` задаёт длительность аккордов, сердцебиения и мурчания; без него они идут до конца кадра. Короткие эффекты имеют свою длительность.

Просмотр без озвучки; вместо `<моменты>` перечислите нужные секунды:

```sh
go run ./cmd/maket stills scenes/moe-tariff-cache.json -o out/moe-stills -silent <моменты>
go run ./cmd/maket render scenes/moe-tariff-cache.json -o out/moe.mp4 -silent
```

## Каст из 70 образов

Во всех четырёх мирах доступны одинаковые имена, каждый рисунок сделан для своего мира. `obj: "warehouse"` и `kind: "warehouse"` выбирают один образ. Роли по умолчанию заданы каталогом и заменяются полем `role` сцены. У новых 70 состояния `idle`, `work`, `broken`, `wait`, `happy`, `sad`: по два циклических кадра, 2/6/4/2/6/3 кадр./с. Значки уже нарисованы; `catch/flinch` в этой поставке отсутствуют. Прежние объекты и их состояния сохранены.

У новых образов масштаб PNG 0.9 со сглаживанием, умножается на `size`. Свет фона действует на девушку и реквизит вместе. Прежний каст сохраняет свой масштаб.
