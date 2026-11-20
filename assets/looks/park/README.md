# Парк: пакет для движка

Все PNG в экранных пикселях (3× логического пикселя). Рисовать только nearest-neighbour, координаты кратны 3 px. Кадр 1080×1920 — окно внутри карты 2160×3840.

## Слои
1. `world/ground.png` — земля целиком: трава, плитка, вода, асфальт, ленты очередей, рельсы горок, кабели, тени статичных объектов. Рисуется первым.
2. `world.json → objects[]` — статичные объекты (здания, деревья, фонари, столбики, ларьки): `sprite`, `pos` (левый верхний угол на карте), `sort_y`. Сортируются по `sort_y` вместе с людьми и динамикой.
3. Динамика: люди, аттракционы, табло, эффекты. `world.json → dynamic_in_reference[]` — где они стоят на эталонной картинке `world/reference.png` (`anchor` = низ-центр, `call` = что это и в каком состоянии).

`world/static_preview.png` — земля + статика без людей и аттракционов, для проверки.

## sprites.json
- `people.<id>` — 24 гостя (`guest00`–`guest23`, нечётные — дети) и 10 ролей персонала (`guard`, `staff`, `cashier`, `operator`, `janitor`, `tech`, `flagger`, `dispatch`, `cook`, `driver`).
  - Кадры: `<view>_<pose>_<i>`. view: `front`, `back`, `side` (смотрит вправо, влево — отражение). pose: `stand_0`, `walk_0..3` (8 кадр./с), `sit_0`, `jump_0..1` (нет у `back`).
  - Эмоции (front, side): `happy_0`, `angry_0`, `cry_0..1`, `sleep_0`, `shock_0`, `wait_0`.
  - У кадра: `feet` (точка ног внутри PNG — ставится в позицию), `hand_r` (правая рука, для предметов), `head` (верх головы, для значков и пузырей).
  - Тень под человеком: `fx/shadow_person.png`, центр в точке ног.
- `items` — предметы в руку: `ticket`, `buzzer`, `phone`, `bag`, `icecream`, `map`, `tray`; браслеты `band_<цвет>`. Позиция = `hand_r + offset_from_hand`.
- `rides`
  - `carousel`: `phase{0..2}_riders{0..6}` — фаза вращения (6 кадр./с, пока работает) и сколько из 6 мест занято.
  - `ferris`: `phase{0..2}_occ{0,3,6,9,12}`.
  - `coaster`: вагоны `car{0..2}_{full|empty}`, едут по `paths.json → coaster`.
- `boards` — пустые табло (`free`, `wait`, `inpark`, `parking`, `ready`) + `value_centre`, шрифт и цвета `ok/warn/bad`. Цифры рисует движок: JetBrains Mono Bold 11 px в 1×, без сглаживания, потом ×3. Ещё `seats{6|8}_{k}`, `light_{green|red}`, `validator_{ok|bad}`, `junction_{on|off}`, `speaker_on_{0..2}`, `speaker_off`.
- `fx` — значки над головой `icon_<вид>_<кадр>` (bang, question, dots ×3, zzz, wait ×2, heart, angry, note, money, ok, no), готовые пузыри `bubble_*`, таблички `plate_РЕМОНТ/ОТКРЫТО/БЕЗ_ОЧЕРЕДИ`, `chain_closed`, `smoke_0..2`, `spark_0..1`. Свой пузырь: `bubble_spec`.

## queues.json
Каждая очередь — список мест от головы к хвосту: `x, y` (точка ног), `view`, `flip`. Движок ставит людей на места и при уходе головы сдвигает всех на одно место (0.45 с, кадры ходьбы). Очереди: `carousel` (змейка, `gate` — вход на аттракцион), `coaster`, `ferris`, `ferris_fastpass` (`priority: true`), `swings`, `bumper`, `security1..3` (`gate` — рамка), `cashier1..4`, `toilets`, `kitchen_order`.

## paths.json
- `nodes` + `edges` — граф дорожек для прохожих (скорость около 60 логических px/с = 180 px/с).
- `coaster.points` — `[x, y, t]` по верху рельса, t 0..1, станция при t = 0.

## cameras.json
Готовые окна 1080×1920 (левый верхний угол): `service_coaster`, `ferris_bumper`, `carousel`, `food_entrance`, `entrance`, `north_lake`. Камера двигается шагом 3 px, без зума.

## Ограничения
- Ленты очередей, рельсы и кабели лежат в земле. Человек, который стоит выше по карте, рисуется поверх них — это нормально для вида сверху.
- Состояние «аттракцион сломан» = дым + табличка «РЕМОНТ» + цепь «ЗАКРЫТО» поверх обычного кадра.
- Эталонный ролик `park-semaphore.mp4` собран только из этого пакета (скрипт-заглушка движка), по нему сверяется картинка.
