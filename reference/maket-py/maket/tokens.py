"""Design tokens for the Maket video style. Change values here, never inline in scenes."""

# --- canvas
WIDTH, HEIGHT, FPS = 1080, 1920, 30
SUPERSAMPLE = 2

# --- safe zones (px, 1080x1920): platform UI lives outside these
SAFE_TOP, SAFE_BOTTOM_Y, SAFE_RIGHT_X, SAFE_LEFT_X = 220, 1500, 940, 70

# --- colour
GROUND = (220, 227, 232)       # floor
GRID = (211, 219, 225)         # floor grid lines
SHADOW = (202, 210, 216)       # contact shadow
INK = (31, 41, 51)             # primary text, caption pill
MUTED = (84, 95, 106)          # status text (5.0:1 on GROUND)
ERROR_TEXT = (176, 38, 46)     # red for text: blocked names, status (5.1:1 on GROUND)
CODE_DIM = (120, 132, 144)     # non-active code lines
CARD = (250, 251, 252)         # code card, token cube
TOKEN_INK = INK

# role -> block colour. One role, one colour, across every video.
ROLES = {
    'producer': (30, 158, 140),   # teal: whoever writes / sends / calls
    'consumer': (124, 92, 230),   # violet: whoever reads / handles
    'buffer':   (240, 162, 2),    # amber: queues, channels, buffers, slots
    'storage':  (47, 111, 219),   # blue: databases, caches, disks
    'external': (91, 107, 122),   # slate: clients, users, third parties
    'error':    (229, 72, 77),    # red: blocked, failed, rejected
    'ok':       (46, 160, 67),    # green: success, ack (use sparingly)
}
# tints for the highlighted code line, by role
CODE_TINT = {
    'producer': (220, 242, 237), 'consumer': (234, 228, 252), 'buffer': (253, 239, 205),
    'storage': (222, 233, 252), 'external': (231, 235, 239), 'error': (252, 226, 227), 'ok': (223, 243, 227),
}
FACE_RIGHT, FACE_LEFT = 0.80, 0.64   # side-face shading multipliers

# --- type (font file, size px)
FONT_LABEL = ('Inter-SemiBold.otf', 30)     # entity names
FONT_STATUS = ('jbm500.ttf', 22)            # status under names, counters
FONT_TOKEN = ('jbm700.ttf', 34)             # value on token cube (<=3 chars)
FONT_TOKEN_SMALL = ('jbm700.ttf', 24)       # 4-5 chars
FONT_CODE = ('jbm500.ttf', 25)
FONT_CAPTION = ('Inter-Bold.otf', 42)

# --- geometry (world units; 1 unit = auto-fit scale, ~50px)
BLOCK = 1.8          # actor cube footprint
BLOCK_H = 1.2
SLOT = 2.0           # slot tray footprint
SLOT_H = 0.45
TOKEN = 1.0          # value cube
CYL_R = 1.0          # storage cylinder radius
CYL_H = 1.4
LABEL_GAP = 34       # px between label lines

# --- layout regions (px)
STAGE_TOP, STAGE_BOTTOM_WITH_CODE, STAGE_BOTTOM_NO_CODE = 230, 1090, 1370
CODE_BOX = (70, 1120, 950, 1380)
CODE_LINE_H = 40
CAPTION_Y = 1458      # centre line of caption pill
CAPTION_H = 84
CAPTION_CENTER_X = 510  # optical centre, left of the right-hand UI column
CAPTION_MAX_WORDS = 4

# --- motion (seconds)
INTRO_STAGGER = 0.10      # between entities rising
INTRO_RISE = 0.45         # one entity rising (ease-out-back)
SEND_DUR = 0.7            # default token flight
SEND_ARC = 1.6            # flight arc height, world units
SHIFT_ARC = 0.8           # arc for moves within one entity (queue shift)
SHAKE_DUR, SHAKE_PX = 0.45, 7
BLOCK_PULSE_HZ = 1.1      # waiting token pulse
FOCUS_DUR = 0.6           # dim-others transition
FOCUS_DIM = 0.4           # remaining opacity of unfocused entities
CAM_DRIFT_ZOOM = 0.03     # slow push-in across the whole video
CAM_FOCUS_ZOOM = 0.03     # extra push-in on focus
CAM_SWAY_PX = 8

# --- sound (gain 0..1, before normalisation)
SFX = {
    'rise': 0.35, 'whoosh': 0.18, 'land': 0.55, 'thud': 0.75, 'buzz': 0.35,
    'pop': 0.5, 'chime': 0.45, 'room_delay': 0.09, 'room_mix': 0.18, 'peak': 0.85,
}
