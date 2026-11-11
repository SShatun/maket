"""python -m maket render scene.json out.mp4 [--no-sfx]
   python -m maket stills scene.json out_dir 1.5 8 12.3"""
import json, os, subprocess, sys, tempfile
from . import tokens as T
from .engine import Renderer
from . import sfx


def load(path):
    with open(path, encoding='utf-8') as f: return json.load(f)


def render(scene_path, out, with_sfx=True):
    r = Renderer(load(scene_path)); sc = r.sc
    tmpdir = tempfile.mkdtemp(prefix='maket_')
    vid = os.path.join(tmpdir, 'video.mp4')
    p = subprocess.Popen(['ffmpeg', '-v', 'error', '-y', '-f', 'rawvideo', '-pix_fmt', 'rgb24', '-s', f'{T.WIDTH}x{T.HEIGHT}', '-r', str(T.FPS), '-i', '-',
                          '-c:v', 'libx264', '-preset', 'veryfast', '-crf', '18', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', vid], stdin=subprocess.PIPE)
    n = int(round(sc.duration * T.FPS))
    for i in range(n):
        p.stdin.write(r.frame(i / T.FPS).tobytes())
        if i % T.FPS == 0: print(f'\r{i / T.FPS:5.1f}s / {sc.duration:.1f}s', end='', file=sys.stderr)
    p.stdin.close()
    if p.wait() != 0: sys.exit('ffmpeg failed')
    print(file=sys.stderr)
    tmp_out = out + '.part.mp4'
    if with_sfx:
        wav = os.path.join(tmpdir, 'sfx.wav'); sfx.build(sc, wav)
        cmd = ['ffmpeg', '-v', 'error', '-y', '-i', vid, '-i', wav, '-map', '0:v', '-map', '1:a', '-c:v', 'copy', '-c:a', 'aac', '-b:a', '160k', '-shortest', tmp_out]
    else:
        cmd = ['ffmpeg', '-v', 'error', '-y', '-i', vid, '-c', 'copy', tmp_out]
    if subprocess.call(cmd) != 0: sys.exit('ffmpeg mux failed')
    os.replace(tmp_out, out)
    print(out)


def stills(scene_path, out_dir, times):
    r = Renderer(load(scene_path)); os.makedirs(out_dir, exist_ok=True)
    for t in times:
        path = os.path.join(out_dir, f'still_{t:06.2f}.png'); r.frame(t).save(path); print(path)


if __name__ == '__main__':
    a = sys.argv[1:]
    if not a or a[0] not in ('render', 'stills'): sys.exit(__doc__)
    if a[0] == 'render': render(a[1], a[2], '--no-sfx' not in a)
    else: stills(a[1], a[2], [float(x) for x in a[3:]])
