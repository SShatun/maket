import json
import sys
from pathlib import Path

import cairosvg

out = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).parent
out.mkdir(parents=True, exist_ok=True)
ink = '#2e2034'
skin = '#ffe8d8'
hair = '#78b8a0'
hair_shadow = '#508878'
white = '#fffcf7'
states = {'idle': (2, 2, True), 'paint': (3, 6, True), 'wait': (3, 3, True), 'happy': (2, 6, True), 'catch': (3, 10, False), 'flinch': (3, 12, False)}

def drawing(state, frame):
    parts = []
    def path(d, fill, stroke=ink, width=5):
        parts.append(f'<path d="{d}" fill="{fill}" stroke="{stroke}" stroke-width="{width}" stroke-linejoin="round" stroke-linecap="round"/>')
    def ellipse(x, y, rx, ry, fill, stroke='none', width=4):
        parts.append(f'<ellipse cx="{x}" cy="{y}" rx="{rx}" ry="{ry}" fill="{fill}" stroke="{stroke}" stroke-width="{width}"/>')
    def rect(x, y, w, h, fill, radius=0, width=5):
        parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{radius}" fill="{fill}" stroke="{ink}" stroke-width="{width}"/>')
    path('M184 274 Q143 291 153 377 Q153 444 182 461 L203 365 Z', hair)
    path('M329 277 Q362 307 352 385 L337 451 L316 432 L316 337 Z', hair)
    path('M163 334 Q157 389 179 434', 'none', hair_shadow, 4)
    path('M337 335 L336 406', 'none', hair_shadow, 4)
    rect(225, 496, 19, 56, skin, 3)
    rect(278, 496, 19, 56, skin, 3)
    rect(220, 549, 29, 11, '#463250', 5)
    rect(273, 549, 29, 11, '#463250', 5)
    path('M216 444 L303 444 L309 510 L211 510 Z', '#403454')
    path('M220 397 Q207 396 200 420 L200 462 Q206 474 219 474 L302 474 Q318 467 319 454 L317 422 Q314 400 299 397 Z', '#cecece')
    path('M230 402 L233 437 M289 403 L285 437', 'none', white, 5)
    path('M228 427 L291 427 L301 489 L218 489 Z', '#ececec')
    rect(242, 450, 36, 24, '#a4a4a4', 4, 3)
    ellipse(260, 330, 91, 75, skin, ink, 6)
    path('M329 372 Q307 401 269 404', 'none', '#f6c4b6', 4)
    path('M161 300 Q163 229 243 225 Q327 218 358 296 L333 309 L317 299 L298 329 L279 300 L260 329 L241 300 L222 329 L202 300 L185 330 Z', hair)
    path('M163 298 L184 310 L186 358 L171 408 L158 364 Z', hair)
    path('M335 299 L355 290 L361 364 L349 408 L332 355 Z', hair)
    path('M196 278 Q250 226 318 278', 'none', '#c2dacf', 9)
    path('M170 253 Q145 214 206 196 Q280 167 330 209 Q358 226 341 252 Z', '#a4a4a4')
    path('M171 249 Q250 235 341 248 L340 260 Q250 252 175 267 Z', '#cecece')
    path('M253 194 L258 183 L269 186 L266 196', '#cecece', ink, 4)
    ellipse(309, 228, 5, 5, '#96c8ff')
    ellipse(294, 218, 3, 3, '#ffce46')
    closed = state in ('happy', 'flinch') or (state == 'idle' and frame == 1)
    for x in (220, 300):
        if closed:
            path(f'M{x-14} 348 Q{x} 330 {x+14} 348', 'none', ink, 4)
        else:
            ellipse(x, 351, 21, 28, white)
            ellipse(x, 353, 17, 24, '#3c6ec8')
            ellipse(x, 356, 13, 20, '#2e5597')
            ellipse(x+1, 364, 11, 14, '#93addd')
            ellipse(x, 352, 8, 13, '#101e2c')
            path(f'M{x-20} 343 Q{x-14} 318 {x+9} 330 Q{x+17} 333 {x+20} 343', 'none', ink, 7)
            ellipse(x-6, 339, 8, 9, white)
            ellipse(x+9, 362, 4, 4, white)
            path(f'M{x-5} 379 Q{x} 381 {x+5} 379', 'none', ink, 2)
    if state == 'wait':
        path('M253 392 Q260 388 267 392', 'none', ink, 3)
        path('M344 309 Q334 327 344 331 Q353 326 344 309', '#96c8ff', ink, 2)
    else:
        path('M248 389 Q252 399 260 391 Q268 399 272 389', 'none', ink, 3)
    if state == 'happy':
        ellipse(195, 375, 9, 4, '#f6c4b6')
        ellipse(323, 375, 9, 4, '#f6c4b6')
        for x, y in [(160, 220+frame*5), (369, 283-frame*5)]:
            path(f'M{x} {y-9} L{x+3} {y-3} L{x+9} {y} L{x+3} {y+3} L{x} {y+9} L{x-3} {y+3} L{x-9} {y} L{x-3} {y-3} Z', '#ffce46', 'none')
    rect(197, 413, 20, 50, '#cecece', 9)
    ellipse(207, 463, 9, 9, skin, ink)
    ellipse(193, 466, 31, 22, '#be8250', ink)
    ellipse(207, 470, 6, 8, skin, ink, 3)
    for x, y, color in [(175, 460, '#96c8ff'), (186, 452, '#ffce46'), (199, 452, '#ff7896'), (178, 473, '#78b8a0')]:
        ellipse(x, y, 5, 5, color)
    raised = state == 'paint'
    bx = 332 + (frame-1)*5 if raised else 330
    by = 415 + (frame%2)*3 if raised else 450
    path(f'M309 415 Q322 414 {bx-8} {by-9} L{bx+4} {by-2} Q324 452 309 457 Z', '#cecece')
    path(f'M{bx-6} {by+16} L{bx+12} {by-35}', 'none', ink, 8)
    path(f'M{bx-6} {by+16} L{bx+12} {by-35}', 'none', '#be8250', 4)
    path(f'M{bx+9} {by-30} L{bx+14} {by-44}', 'none', '#a4a4a4', 7)
    path(f'M{bx+12} {by-40} Q{bx+6} {by-47} {bx+22} {by-58} Q{bx+25} {by-43} {bx+12} {by-40}', '#96c8ff', ink, 3)
    ellipse(bx-3, by+2, 9, 9, skin, ink)
    if state == 'wait':
        for j in range(3):
            ellipse(370+j*10, 210-(frame==j)*3, 3, 3, ink)
    body = ''.join(parts)
    if state == 'catch':
        sx, sy = [(1.05, .94), (.96, 1.04), (1, 1)][frame]
        body = f'<g transform="translate(260 560) scale({sx} {sy}) translate(-260 -560)">{body}</g>'
    if state == 'flinch':
        body = f'<g transform="translate({[-4,4,0][frame]} 0)">{body}</g>'
    if state == 'happy':
        body = f'<g transform="translate(0 {-frame*5})">{body}</g>'
    return f'<svg xmlns="http://www.w3.org/2000/svg" width="1040" height="1200" viewBox="0 0 520 600">{body}</svg>'

for state, (frames, fps, loop) in states.items():
    for frame in range(frames):
        cairosvg.svg2png(bytestring=drawing(state, frame).encode(), write_to=str(out / f'{state}_{frame}.png'))
entry = {'size': [1040, 1200], 'anchor': [520, 1120], 'top': [520, 300], 'scale': 2, 'rest': 'idle', 'tint': True, 'source': 'source.py', 'states': {name: {'frames': frames, 'fps': fps, 'loop': loop} for name, (frames, fps, loop) in states.items()}}
(out / 'entry.json').write_text(json.dumps(entry, indent=1)+'\n')
