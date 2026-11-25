# Пиксели

## Что есть

- Вид: плоская пиксельная графика. Место — картинка стилистики или цвет стены и пола. Стилистик несколько, у каждой свои персонажи и фоны.
- Персонажи: 70 общих образов (сервисы, склад, посылка, ценник, люди) и образы стилистики. Размер от 0.2 до 4: от толпы мелких фигур до одной во весь кадр, обрезанной краем.
- Персонаж меняет позу и лицо, цвет роли, значок над головой (`bad`, `ok`, `question`, `wait`) и подпись-статус под ногами.
- Кадры: общий, средний и крупный план, наезд камеры `zoom`, ходьба `walk`. Склейка — простая смена кадра, без переходов.
- Надписи и данные прямо в кадре:
  - крупное число или слово `text` (100–150 px);
  - табличка `panel`: строки лога или таблица «поле → значение», значения меняются во времени;
  - стрелка, черта или пунктир `line`, с подписью, в том числе от персонажа к персонажу;
  - большое число в рамке `number`;
  - притушить всё, кроме целей, `focus`;
  - неверная догадка крупно и её зачёркивание `guess` / `answer`;
  - схема из узлов и стрелок поверх кадров `diagram` с осью времени.
- Толпа: фигуры рядами в прямоугольнике. Толпа в действии такая же, как у Орков: строй, поток, рой. Вахтёр пропускает её по одному через шлагбаум, над героем можно показать табличку попыток со счётчиком.
- Нет: настроения через фон и свет, переходов через чёрное и белое, диалога разными голосами.

## Стилистики

У Пикселей может быть много стилистик: свои персонажи, цвета и фоны мест под тему. Стилистика по умолчанию — `assets/live`, остальные — папки `assets/looks/<имя>/`. Сцена выбирает стилистику полем `look`.

`go run ./cmd/maket styles` печатает для каждой стилистики:
- `about` — о чём она и какие у неё персонажи;
- `palette` — имена цветов для `wall` и `ground` в `place`;
- `backdrops` — имена картинок мест для `"place": "<имя>"`, у каждой своя линия пола.

Стилистику выбирают по `about`: та, чьи персонажи и места ближе к теме истории, выразительнее. Персонажи стилистики и их состояния — `go run ./cmd/maket char list -look <имя>`.

## Кадры

Ролик в Пикселях строится из кадров `shots` (формат — `docs/scene.md`, раздел «Последовательность кадров»). Кадры каждого ролика придумываются под его историю: место, кто в кадре, общий план или крупный, что делают. Одинаковой раскладки на все ролики нет.

- Общий план: дальний пол (`floor` 600–900), толпа `crowd` или несколько фигур `size` 0.3–0.8, дальние выше и мельче.
- Средний план: два-три участника `size` 1–1.6, пол около 1450.
- Крупный план: один участник `size` 2–3, пол у нижнего края, число или вопрос надписью над ним.
- Склейка — простая смена кадра, без затемнений и переходов (`assets/design/motion.md`). Камера в кадре только наезжает (`zoom`).

Пока у стилистики по умолчанию нет картинок мест, место задают цветами: `{"wall": "warehouse", "floor": 1460, "ground": "shelf"}` или RGB.

## Толпа в действии

Для нескольких запросов у лимита используйте `crowd.behavior`. Это тот же механизм, что в Орках: `route`, `stream`, `formation`, `swarm`, часть толпы `part`, события `state`, `role`, `badge`, `admit` и `tries`. Пиксели дают ему свои рисунки и размеры. Обычная массовка с `box`, `rows` и диапазоном `size` остаётся доступна без `behavior`.

Координаты здесь в пикселях кадра 1080×1920. Размер одной фигуры задаётся `size` у записи `cast`, например 0.28; внутри `crowd` диапазон размеров для поведения не нужен. Для схемы хватает 3–6 фигур. У `formation` задайте `at`, `cols`, `rows` или `count`, `step: [dx, dy]` между фигурами. `speed` у `formation` и `stream` измеряется в пикселях кадра в секунду; у `route` в кругах в секунду. Без своих размеров шаг строя 132×144, скорость 360 px/с. `spread`, `depth`, `radius`, `lane`, `from` и `to` тоже в пикселях кадра.

```json
{"id": "Q", "kind": "service", "size": 0.28, "at": [440, 1260],
 "crowd": {"behavior": "formation", "count": 4, "cols": 2, "step": [160, 180],
           "part": {"count": 2, "role": "error", "badges": [[0, "bad"]]}}}
```

`id` такой записи обозначает всю толпу. `part: true` у `state`, `role` или `badge` меняет только выбранную часть. В строю это первые `part.count` фигур со стороны прохода, в остальных движениях выбор зависит от `seed`. Участники без нарисованного `walk` двигаются со своей спокойной анимацией. Знаки Пикселей: `bad`, `ok`, `question`, `wait`; `badge: null` убирает знак.

Вахтёр `lock` служит проходом: `deny` держит очередь, `admit` поочерёдно отправляет её участников через шлагбаум и переключает его между `allow` и `deny`. `gate` указывает вахтёра, `from` строй, `rate` число проходов в секунду, `to` место назначения. Без `admit` строй остаётся на месте.

```json
{"at": "p3:пропускает", "do": "admit", "gate": "GATE", "from": "Q", "rate": 0.6, "to": "SERVICE"}
{"at": "p2", "do": "tries", "target": "CLIENT", "text": "{n}/3", "count": [1, 3, 4], "badge": "wait", "until": "p3"}
```

`tries` показывает попытки над одним участником, `count: [от, до, секунды]` меняет целое число ступенями. `wait` добавляет часы и убывающую полоску до следующей попытки. Новая табличка заменяет предыдущую; без `until` она остаётся до склейки. Пример целиком: `scenes/pixel-rate-limit.json`.

## Каст из 70 образов

Во всех четырёх мирах доступны одинаковые имена, каждый рисунок сделан для своего мира. `obj: "warehouse"` и `kind: "warehouse"` выбирают один образ. Роли по умолчанию заданы каталогом и заменяются полем `role` сцены. У новых 70 состояния `idle`, `work`, `broken`, `wait`, `happy`, `sad`: по два циклических кадра, 2/6/4/2/6/3 кадр./с. Значки уже нарисованы; `catch/flinch` в этой поставке отсутствуют. Прежние объекты и их состояния сохранены.

Новые и базовые образы используют одинаковый масштаб PNG 1 с nearest-neighbor: при `size: 1` логический пиксель занимает 12 px. Поле `size` умножает этот масштаб.


<!-- characters:start -->
| kind | кто | синонимы | состояния | происхождение |
|---|---|---|---|---|
| `alert` | алерт | alert, алерт | broken, happy, idle, sad, wait, work | дизайнер |
| `avail` | доступность товара | avail, доступность товара | broken, happy, idle, sad, wait, work | дизайнер |
| `baby` | малыш | baby, малыш | broken, happy, idle, sad, wait, work | дизайнер |
| `balancer` | балансировщик | balancer, балансировщик | broken, happy, idle, sad, wait, work | дизайнер |
| `batch` | пакетная обработка | batch, пакетная обработка | broken, happy, idle, sad, wait, work | дизайнер |
| `bloom` | фильтр Блума | bloom, фильтр Блума | broken, happy, idle, sad, wait, work | дизайнер |
| `branch` | росток в горшке | git, коммит, branch, ветка | catch, flinch, grow, idle, pushed | дизайнер |
| `breaker` | предохранитель запросов | breaker, предохранитель запросов | broken, happy, idle, sad, wait, work | дизайнер |
| `btree` | B-дерево | btree, B-дерево | broken, happy, idle, sad, wait, work | дизайнер |
| `bucket` | бакет | bucket, бакет | broken, happy, idle, sad, wait, work | дизайнер |
| `budget` | бюджет | budget, бюджет | broken, happy, idle, sad, wait, work | дизайнер |
| `cache` | рюкзак с карманом | рюкзак, готовые ответы, cache, redis, кэш | catch, flinch, give, idle, miss | дизайнер |
| `card` | карточка товара | card, карточка товара | broken, happy, idle, sad, wait, work | дизайнер |
| `channel` | канал | channel, канал | broken, happy, idle, sad, wait, work | дизайнер |
| `client` | клиент | client, клиент | broken, happy, idle, sad, wait, work | дизайнер |
| `context` | контекст отмены | context, контекст отмены | broken, happy, idle, sad, wait, work | дизайнер |
| `courier` | курьер | courier, курьер | broken, happy, idle, sad, wait, work | дизайнер |
| `cpu` | процессор | cpu, процессор | broken, happy, idle, sad, wait, work | дизайнер |
| `cron` | планировщик | cron, планировщик | broken, happy, idle, sad, wait, work | дизайнер |
| `customs` | таможенник | customs, таможенник | broken, happy, idle, sad, wait, work | дизайнер |
| `db` | шкаф-картотека | картотека, таблица, хранилище, postgres, db, database, база | catch, empty, flinch, give, idle | дизайнер |
| `deploy` | деплой | deploy, деплой | broken, happy, idle, sad, wait, work | дизайнер |
| `developer` | человек за ноутбуком | разработчик, сотрудник, инженер, developer, человек за ноутбуком | catch, flinch, happy, idle, surprised, wait | добавлен |
| `dialog` | окошко Yes/No | диалог, вопрос, подтверждение, dialog, prompt, окошко | catch, closed, flinch, idle, impatient | дизайнер |
| `disk` | диск | disk, диск | broken, happy, idle, sad, wait, work | дизайнер |
| `dlq` | очередь ошибок | dlq, очередь ошибок | broken, happy, idle, sad, wait, work | дизайнер |
| `docs` | документация | docs, документация | broken, happy, idle, sad, wait, work | дизайнер |
| `duty` | таможенная пошлина | duty, таможенная пошлина | broken, happy, idle, sad, wait, work | дизайнер |
| `err500` | ошибка 500 | err500, ошибка 500 | broken, happy, idle, sad, wait, work | дизайнер |
| `eta` | срок доставки | eta, срок доставки | broken, happy, idle, sad, wait, work | дизайнер |
| `external` | здание чужого сервиса | внешняя система, партнёр, апи, external, api, чужой сервис | answer, catch, closed, flinch, idle | дизайнер |
| `flag` | флаг функции | flag, флаг функции | broken, happy, idle, sad, wait, work | дизайнер |
| `gc` | сборщик мусора | gc, сборщик мусора | broken, happy, idle, sad, wait, work | дизайнер |
| `gor` | горутина | gor, горутина | broken, happy, idle, sad, wait, work | дизайнер |
| `hash` | хеш | hash, хеш | broken, happy, idle, sad, wait, work | дизайнер |
| `heap` | куча | heap, куча | broken, happy, idle, sad, wait, work | дизайнер |
| `iface` | интерфейс | iface, интерфейс | broken, happy, idle, sad, wait, work | дизайнер |
| `leader` | лидер | leader, лидер | broken, happy, idle, sad, wait, work | дизайнер |
| `limiter` | ограничитель запросов | limiter, ограничитель запросов | broken, happy, idle, sad, wait, work | дизайнер |
| `list` | связный список | list, связный список | broken, happy, idle, sad, wait, work | дизайнер |
| `lock` | вахтёр со списком и шлагбаумом | вахтёр, проверка, хук, доступ, lock, guard, замок | allow, catch, check, deny, flinch, idle | дизайнер |
| `log` | лог | log, лог | broken, happy, idle, sad, wait, work | дизайнер |
| `mail` | почта | mail, почта | broken, happy, idle, sad, wait, work | дизайнер |
| `market` | маркетплейс | market, маркетплейс | broken, happy, idle, sad, wait, work | дизайнер |
| `metric` | метрика | metric, метрика | broken, happy, idle, sad, wait, work | дизайнер |
| `migration` | миграция | migration, миграция | broken, happy, idle, sad, wait, work | дизайнер |
| `mr` | запрос на слияние | mr, запрос на слияние | broken, happy, idle, sad, wait, work | дизайнер |
| `mutex` | мьютекс | mutex, мьютекс | broken, happy, idle, sad, wait, work | дизайнер |
| `oncall` | дежурный с рацией | oncall, дежурный с рацией | broken, happy, idle, sad, wait, work | дизайнер |
| `packet` | сетевой пакет | packet, сетевой пакет | broken, happy, idle, sad, wait, work | дизайнер |
| `parcel` | картонная посылка с лицом | товар, заказ, коробка, parcel, item, посылка | catch, flinch, happy, idle, sad, wait | дизайнер |
| `person` | человек с телефоном | пользователь, дежурный, клиент, user, person, человек | catch, flinch, happy, idle, surprised, wait | дизайнер |
| `plane` | самолёт | plane, самолёт | broken, happy, idle, sad, wait, work | дизайнер |
| `pointer` | указатель | pointer, указатель | broken, happy, idle, sad, wait, work | дизайнер |
| `product` | продакт со свитком PRD | product, продакт со свитком PRD | broken, happy, idle, sad, wait, work | дизайнер |
| `pvz` | пункт выдачи | pvz, пункт выдачи | broken, happy, idle, sad, wait, work | дизайнер |
| `queue` | конвейер с головой-мотором | конвейер, буфер, очередь задач, queue, buffer, очередь | catch, flinch, full, run, stop | дизайнер |
| `replica` | реплика | replica, реплика | broken, happy, idle, sad, wait, work | дизайнер |
| `resync` | пересинхронизация | resync, пересинхронизация | broken, happy, idle, sad, wait, work | дизайнер |
| `retry` | повтор запроса | retry, повтор запроса | broken, happy, idle, sad, wait, work | дизайнер |
| `select` | выбор канала | select, выбор канала | broken, happy, idle, sad, wait, work | дизайнер |
| `seller` | продавец | seller, продавец | broken, happy, idle, sad, wait, work | дизайнер |
| `semaphore` | семафор | semaphore, семафор | broken, happy, idle, sad, wait, work | дизайнер |
| `service` | машинка с экраном-лицом и счётами | расчёт, джоба, воркер, обработчик, service, worker, сервис | bad, catch, count, flinch, idle, skip, wait | дизайнер |
| `session` | терминал с глазами-курсорами | терминал, агент, консоль, session, terminal, сессия | catch, flinch, idle, sleep, type, wait | дизайнер |
| `shard` | шард | shard, шард | broken, happy, idle, sad, wait, work | дизайнер |
| `shipping` | способ доставки | shipping, способ доставки | broken, happy, idle, sad, wait, work | дизайнер |
| `slice` | срез | slice, срез | broken, happy, idle, sad, wait, work | дизайнер |
| `stack` | стек | stack, стек | broken, happy, idle, sad, wait, work | дизайнер |
| `stock` | остаток товара | stock, остаток товара | broken, happy, idle, sad, wait, work | дизайнер |
| `struct` | структура | struct, структура | broken, happy, idle, sad, wait, work | дизайнер |
| `tariff` | ценник со знаком рубля | цена, прайс, стоимость, ценник, тарификация, price, tariff, тариф | catch, flinch, happy, idle | добавлен |
| `template` | папка с карточками | папка, настройки продавца, способы доставки, template, шаблон | catch, empty, flinch, full, idle | дизайнер |
| `ticket` | задача | ticket, задача | broken, happy, idle, sad, wait, work | дизайнер |
| `timeout` | таймаут | timeout, таймаут | broken, happy, idle, sad, wait, work | дизайнер |
| `topic` | лента писем с читателем | лента, kafka, сообщения, topic, топик | catch, flinch, flow, idle, lag | дизайнер |
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
