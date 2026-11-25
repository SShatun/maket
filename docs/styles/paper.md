# Стиль paper («Бумага»)

## Что есть

- Вид: фигуры, вырезанные из бумаги, на рисованных фонах. Те же 70 образов и 13 своих.
- Персонажи въезжают с края и ходят подскоками. Кто говорит, у того двигается рот и звучит бормотание.
- Диалог в два голоса: у каждого персонажа свой голос (`paper-loud`, `paper-shy`).
- Места: день, ночь, закат, авария, класс с доской, офис днём, вечером и ночью.
- Доска в классе: строки пишутся мелом по буквам — лог, статус, короткий расчёт.
- Титул — лист на скотче, свободные надписи рукописным шрифтом, вспышка, склейка через чёрное или белое.
- Схема из узлов и стрелок `diagram` поверх кадров.
- Звук: перебор гитары на склейках, мел, бормотание.
- Нет: толпы, значков и статусов над персонажами, табличек, стрелок, наезда камеры.

## Сцена и декорации

`"style": "paper"`, без `look`. Каталог находится в `assets/looks/paper/`, используется общий загрузчик спрайтов. Формат тот же, что у других миров: `phrases`, `shots`, `cast`, `do`, время в секундах или `p2`, `p2:слово`, `p2+0.4`. Пример: `scenes/paper-dialogue.json`.

| `place` | Для чего |
|---|---|
| `day` | обычная сцена, титул, развязка |
| `night` | ожидание, таймаут, сон |
| `sunset` | конец дня, вывод |
| `alarm` | авария, всё упало |
| `classroom` | объяснение у доски |

Фоны 1080×1920, рекомендуемая точка ног y=1560. Имена, линия стояния и прямоугольник доски сохранены в `bg/backgrounds.json`, список мест также доступен через `go run ./cmd/maket styles`. Свет и тени поверх рисунков не добавляются.

## Персонажи и реплики

```json
{"id": "W", "obj": "woman", "voice": "paper-loud", "at": [290, 1560], "size": 1.1, "enter": "left"}
{"text": "Склад, ты работаешь?", "caption": "Склад, ты работаешь?", "speaker": "W"}
```

`obj` и `kind` равнозначны. `speaker` у фразы указывает `id` участника текущего кадра. Пока виден этот субтитр, включая обычные 0.35 с после конца фразы, у говорящего играет `talk`, а у образов без `talk` состояние `idle` временно заменяется на `work`. Остальные состояния, включая `broken`, `sad` и `wait`, продолжают играть во время речи без перезапуска. После реплики снова действует его обычная шкала `state/states`: изменения состояния во время речи сохраняются. Если реплика пересекает склейку, её `speaker` должен присутствовать и в следующем кадре. Без `speaker` фраза остаётся закадровой, без анимации рта и бормотания.

`state`, `states`, `role`, `roles`, `label`, `flip` доступны для каждого участника. Роль по умолчанию берётся из каталога, иначе `external`; сцена может заменить её. Перекрашиваются только нейтральные пиксели r=g=b. Объекты с `tint: false` сохраняют цвет. Значки состояний уже нарисованы. Дополнительные `badges`, `status`, `crowd` не поддерживаются; надписи задаются через `text`.

PNG 880×960 представляют логический холст 440×480. `size` задаёт масштаб PNG в кадре: по умолчанию 1.1 для нескольких участников и 1.3 для одного. Поле `scale: 2` исходного каталога описывает плотность рисунка и не делит масштаб сцены. Допустимы значения 0.2…4. Масштабирование плавное, со сглаживанием.

В начале кадра персонаж въезжает за 0.8 с из ближайшего края. `enter: "left"` или `"right"` выбирает край, `"none"` сразу ставит его в точку. Позиции меняются 12 раз в секунду, с подскоком 12 px; состояния продолжают играть со своим `fps`. `walk` в этом мире тоже перемещает вырезку подскоками. Нарисованный прыжок состояния `happy` не усиливается отдельной анимацией.

## События

| `do` | Поля | Действие |
|---|---|---|
| `state` | `target`, `state`, `hold` | смена состояния, разовое без `hold` возвращается в `rest` |
| `role` | `target`, `role` | смена роли |
| `walk` | `target`, `to: [x, y]`, `dur` | перемещение подскоками, по умолчанию `dur` 0.8 с |
| `enter` | `target`, `side`, `dur` | повторный въезд из `left` или `right` к исходной точке, по умолчанию 0.8 с |
| `bg` | `name`, `dur` | смена фона через чёрное 0.3 с; переход в `classroom` через белое 0.15 с |
| `chalk` | `lines`, `cps`, `until` | текст на доске `classroom`, по умолчанию 18 символов/с |
| `title` | `episode`, `lines`, `until` | лист на скотче падает сверху за 0.4 с; одна-две короткие строки |
| `text` | `text`, `pos`, `size`, `tone`, `until` | надпись Pangolin, по умолчанию 44 px |
| `cut` | `color`, `dur` | переход через чёрное 0.3 с или белое 0.15 с; `at` задаёт середину |
| `flash` | `dur` | белая вспышка с `at`, по умолчанию 0.15 с |
| `sfx` | `name`, `dur` | явный звук |

Новый кадр автоматически начинается через чёрное, класс через белое. Явный `cut` в этот же момент заменяет автоматический переход. Первый кадр появляется сразу. Состояния и роли сохраняются только внутри кадра; `until` ограничивает текст и звук, для `state/role` задавайте следующий шаг явно.

Доска занимает [74, 274, 1006, 876]. Текст начинается в (120, 340), Pangolin 60 px, цвет (245, 245, 235), шаг строк 74 px. Строки появляются последовательно по буквам; новый `chalk` заменяет предыдущий текст, смена фона очищает доску. Слишком длинные строки или текст вне доски вызывают ошибку. Текст виден до `until`, следующего `chalk` или склейки.

```json
{"at": "p3", "do": "chalk", "lines": ["status: 500", "reason: timeout"], "cps": 18}
{"at": 0, "do": "title", "episode": 1, "lines": ["ТИХИЙ СКЛАД"], "until": 1.2}
```

Титул использует Pangolin 44 px для «ЭПИЗОД N», 104 и 92 px для названия. Субтитры: Inter Bold 50, белые, обводка 6 px, центр y=1760, караоке по словам. Pangolin и SIL OFL 1.1 находятся в `assets/fonts/`; источник: [Google Fonts](https://github.com/google/fonts/tree/main/ofl/pangolin).

## Голоса

Бумага озвучивается Fish Audio двумя фиксированными голосами публичной библиотеки: `paper-loud` (`e857469fd8a44624a842bc13808d315f`) и `paper-shy` (`23a1470b05b74c5ebaa09572845ec46e`). Модель берётся из `fish_model`; установщик задаёт `s2.1-pro-free`. Имя выбранной модели передаётся в обязательном заголовке `model`. Нужен ключ Fish, см. [установку](../install.md).

В `assets/looks/paper/look.json` заданы `voice_backend: "fish"`, голос по умолчанию `voice: "paper-loud"` и список `voices`. Бэкенд выбирается в порядке `MAKET_VOICE_BACKEND` > `voice_backend` стиля > `maket.local.json` > `maket.json` > Qwen. Другие стили сохраняют свои настройки.

Каждому персонажу задайте `voice` с именем папки из `voices/`, а его реплике `speaker` с тем же `id`:

```json
{"id": "W", "obj": "woman", "voice": "paper-loud", "at": [215, 1560]}
{"id": "S", "obj": "warehouse", "voice": "paper-shy", "at": [760, 1560]}
{"text": "Склад, ты работаешь?", "speaker": "W"}
{"text": "Я просто тихо умираю.", "speaker": "S"}
```

Голос закреплён за `id` на всю сцену: достаточно указать его в одном появлении, повторные указания должны совпадать. Без `speaker` или без назначенного ему `voice` используется голос сцены, затем стиля, затем конфигурации. `-voice` заменяет только этот голос по умолчанию, сохраняя голоса персонажей.

В `voices/<имя>/voice.json` библиотечный голос задаётся через `reference_id`, без `ref_audio` и `ref_text`. Поля `stress` и `spell` сохраняют формат обычного голоса; Fish применяет `spell` выбранного персонажа, но не использует словарь ударений и RUAccent. Другие бэкенды сообщают, что такой голос существует только в Fish. Каждая фраза синтезируется отдельно с её `reference_id`; паузы сцены сохраняются, времена слов берутся из локального Whisper. Это работает в `stress`, `voice`, `stills` и `render`; проверка `hear` использует ту же собранную дорожку.

## Звук

`strum` на каждой склейке и смене фона: акустический перебор Karplus–Strong, аккорды G, D, Em, C, струны через 18 мс. `babble` под репликой указанного `speaker`: слоги 70–140 мс, основной тон с двумя гармониками, база 260 Гц для `person` и группы `people` (`woman`, `baby`, `oncall`, `product`, `seller`, `courier`, `customs`), 180 Гц для остальных. `chalk` звучит только пока пишутся буквы: фильтрованный шум 30 мс через каждые 90 мс.

Событие `state` добавляет `pop` для `dialog.closed`, `whoosh` и `rise` для `pushed` и `deploy.work`, `chime` для `happy`. Явный `sfx` также принимает `strum`, `babble`, `chalk`, `pop`, `whoosh`, `rise`, `land`, `thud`, `buzz`, `chime`. `sound: false` отключает звук конкретного события; `-no-sfx` отключает все эффекты, сохраняя голос. `-silent` отключает только голос. Микс использует общую нормализацию, комнатное эхо и приглушение под голосом.

## Каталог

13 базовых образов и 70 новых. Новые: `idle`, `work`, `broken`, `wait`, `happy`, `sad`, по два циклических кадра, соответственно 2/6/4/2/6/3 кадр./с. Общих `catch/flinch` у новых 70 нет; используйте только состояния из каталога.

`go run ./cmd/maket char list -look paper` и `go run ./cmd/maket char sheet woman -look paper -ref warehouse,err500 -o out/paper-cast.png`.

<!-- characters:start -->
| kind | кто | синонимы | состояния | происхождение |
|---|---|---|---|---|
| `alert` | алерт | alert, алерт | broken, happy, idle, sad, wait, work | дизайнер |
| `avail` | доступность товара | avail, доступность товара | broken, happy, idle, sad, wait, work | дизайнер |
| `baby` | малыш | baby, малыш | broken, happy, idle, sad, wait, work | дизайнер |
| `balancer` | балансировщик | balancer, балансировщик | broken, happy, idle, sad, wait, work | дизайнер |
| `batch` | пакетная обработка | batch, пакетная обработка | broken, happy, idle, sad, wait, work | дизайнер |
| `bloom` | фильтр Блума | bloom, фильтр Блума | broken, happy, idle, sad, wait, work | дизайнер |
| `branch` | росток в горшке | branch, росток в горшке | catch, flinch, grow, idle, pushed, talk | дизайнер |
| `breaker` | предохранитель запросов | breaker, предохранитель запросов | broken, happy, idle, sad, wait, work | дизайнер |
| `btree` | B-дерево | btree, B-дерево | broken, happy, idle, sad, wait, work | дизайнер |
| `bucket` | бакет | bucket, бакет | broken, happy, idle, sad, wait, work | дизайнер |
| `budget` | бюджет | budget, бюджет | broken, happy, idle, sad, wait, work | дизайнер |
| `cache` | холодильник | cache, холодильник | catch, flinch, give, idle, miss, talk | дизайнер |
| `card` | карточка товара | card, карточка товара | broken, happy, idle, sad, wait, work | дизайнер |
| `channel` | канал | channel, канал | broken, happy, idle, sad, wait, work | дизайнер |
| `client` | клиент | client, клиент | broken, happy, idle, sad, wait, work | дизайнер |
| `context` | контекст отмены | context, контекст отмены | broken, happy, idle, sad, wait, work | дизайнер |
| `courier` | курьер | courier, курьер | broken, happy, idle, sad, wait, work | дизайнер |
| `cpu` | процессор | cpu, процессор | broken, happy, idle, sad, wait, work | дизайнер |
| `cron` | планировщик | cron, планировщик | broken, happy, idle, sad, wait, work | дизайнер |
| `customs` | таможенник | customs, таможенник | broken, happy, idle, sad, wait, work | дизайнер |
| `db` | картотечный шкаф | db, картотечный шкаф | catch, empty, flinch, give, idle, talk | дизайнер |
| `deploy` | деплой | deploy, деплой | broken, happy, idle, sad, wait, work | дизайнер |
| `dialog` | окно Yes / No | dialog, окно Yes / No | catch, closed, flinch, idle, impatient, talk | дизайнер |
| `disk` | диск | disk, диск | broken, happy, idle, sad, wait, work | дизайнер |
| `dlq` | очередь ошибок | dlq, очередь ошибок | broken, happy, idle, sad, wait, work | дизайнер |
| `docs` | документация | docs, документация | broken, happy, idle, sad, wait, work | дизайнер |
| `duty` | таможенная пошлина | duty, таможенная пошлина | broken, happy, idle, sad, wait, work | дизайнер |
| `err500` | ошибка 500 | err500, ошибка 500 | broken, happy, idle, sad, wait, work | дизайнер |
| `eta` | срок доставки | eta, срок доставки | broken, happy, idle, sad, wait, work | дизайнер |
| `external` | офис чужой компании | external, офис чужой компании | answer, catch, closed, flinch, idle, talk | дизайнер |
| `flag` | флаг функции | flag, флаг функции | broken, happy, idle, sad, wait, work | дизайнер |
| `gc` | сборщик мусора | gc, сборщик мусора | broken, happy, idle, sad, wait, work | дизайнер |
| `gor` | горутина | gor, горутина | broken, happy, idle, sad, wait, work | дизайнер |
| `hash` | хеш | hash, хеш | broken, happy, idle, sad, wait, work | дизайнер |
| `heap` | куча | heap, куча | broken, happy, idle, sad, wait, work | дизайнер |
| `iface` | интерфейс | iface, интерфейс | broken, happy, idle, sad, wait, work | дизайнер |
| `leader` | лидер | leader, лидер | broken, happy, idle, sad, wait, work | дизайнер |
| `limiter` | ограничитель запросов | limiter, ограничитель запросов | broken, happy, idle, sad, wait, work | дизайнер |
| `list` | связный список | list, связный список | broken, happy, idle, sad, wait, work | дизайнер |
| `lock` | охранник со списком | lock, охранник со списком | allow, catch, check, deny, flinch, idle, talk | дизайнер |
| `log` | лог | log, лог | broken, happy, idle, sad, wait, work | дизайнер |
| `mail` | почта | mail, почта | broken, happy, idle, sad, wait, work | дизайнер |
| `market` | маркетплейс | market, маркетплейс | broken, happy, idle, sad, wait, work | дизайнер |
| `metric` | метрика | metric, метрика | broken, happy, idle, sad, wait, work | дизайнер |
| `migration` | миграция | migration, миграция | broken, happy, idle, sad, wait, work | дизайнер |
| `mr` | запрос на слияние | mr, запрос на слияние | broken, happy, idle, sad, wait, work | дизайнер |
| `mutex` | мьютекс | mutex, мьютекс | broken, happy, idle, sad, wait, work | дизайнер |
| `oncall` | дежурный с рацией | oncall, дежурный с рацией | broken, happy, idle, sad, wait, work | дизайнер |
| `packet` | сетевой пакет | packet, сетевой пакет | broken, happy, idle, sad, wait, work | дизайнер |
| `parcel` | картонная коробка | parcel, картонная коробка | catch, flinch, happy, idle, sad, talk, wait | дизайнер |
| `person` | мальчик в розовой шапке | person, мальчик в розовой шапке | catch, flinch, happy, idle, surprised, talk, wait | дизайнер |
| `plane` | самолёт | plane, самолёт | broken, happy, idle, sad, wait, work | дизайнер |
| `pointer` | указатель | pointer, указатель | broken, happy, idle, sad, wait, work | дизайнер |
| `product` | продакт со свитком PRD | product, продакт со свитком PRD | broken, happy, idle, sad, wait, work | дизайнер |
| `pvz` | пункт выдачи | pvz, пункт выдачи | broken, happy, idle, sad, wait, work | дизайнер |
| `queue` | фургон с коробками | queue, фургон с коробками | catch, flinch, full, run, stop, talk | дизайнер |
| `replica` | реплика | replica, реплика | broken, happy, idle, sad, wait, work | дизайнер |
| `resync` | пересинхронизация | resync, пересинхронизация | broken, happy, idle, sad, wait, work | дизайнер |
| `retry` | повтор запроса | retry, повтор запроса | broken, happy, idle, sad, wait, work | дизайнер |
| `select` | выбор канала | select, выбор канала | broken, happy, idle, sad, wait, work | дизайнер |
| `seller` | продавец | seller, продавец | broken, happy, idle, sad, wait, work | дизайнер |
| `semaphore` | семафор | semaphore, семафор | broken, happy, idle, sad, wait, work | дизайнер |
| `service` | повар со счётами | service, повар со счётами | bad, catch, count, flinch, idle, skip, talk, wait | дизайнер |
| `session` | ЭЛТ-монитор с лицом | session, ЭЛТ-монитор с лицом | catch, flinch, idle, sleep, talk, type, wait | дизайнер |
| `shard` | шард | shard, шард | broken, happy, idle, sad, wait, work | дизайнер |
| `shipping` | способ доставки | shipping, способ доставки | broken, happy, idle, sad, wait, work | дизайнер |
| `slice` | срез | slice, срез | broken, happy, idle, sad, wait, work | дизайнер |
| `stack` | стек | stack, стек | broken, happy, idle, sad, wait, work | дизайнер |
| `stock` | остаток товара | stock, остаток товара | broken, happy, idle, sad, wait, work | дизайнер |
| `struct` | структура | struct, структура | broken, happy, idle, sad, wait, work | дизайнер |
| `template` | папка с карточками | template, папка с карточками | catch, empty, flinch, full, idle, talk | дизайнер |
| `ticket` | задача | ticket, задача | broken, happy, idle, sad, wait, work | дизайнер |
| `timeout` | таймаут | timeout, таймаут | broken, happy, idle, sad, wait, work | дизайнер |
| `topic` | почтовый ящик | topic, почтовый ящик | catch, flinch, flow, idle, lag, talk | дизайнер |
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
