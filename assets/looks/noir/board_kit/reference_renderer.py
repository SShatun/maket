"""Board as a kit: parts + a reference renderer that builds any case from JSON using only the parts.
Port render_case() to the engine; it draws yarn as plain lines (spec in README)."""
import math, random, json, os, shutil
from PIL import Image, ImageDraw, ImageFont
import hard_board as HB
from hardlib import *
W,H=1080,1920
FONT='/usr/share/fonts/opentype/inter/Inter-ExtraBold.otf'
KIT='pkg/board_kit'

# ---------------------------------------------------------------- extra icons (photo area 240x180, same faceted style)
def icon_extra(name,p,r,ln,el,tx):
    r(20,20,260,200,(40,46,58)); p([(20,20),(260,20),(260,70),(20,130)],(66,76,90)); r(20,172,260,200,(24,28,36))
    ST,ST2=HB.STEEL,HB.STEEL2; WD,WD2,WD3=HB.WOOD,HB.WOOD2,HB.WOOD3
    if name=='БАЛАНСЕР':   # signpost with three arms
        r(132,40,146,190,WD2); p([(132,40),(139,40),(139,190),(132,190)],WD)
        for (y,dx,c) in ((52,1,IV),(92,-1,(214,206,186)),(132,1,IV)):
            if dx>0: p([(146,y),(226,y),(244,y+16),(226,y+32),(146,y+32)],c)
            else: p([(132,y),(52,y),(34,y+16),(52,y+32),(132,y+32)],c)
            ln([(150 if dx>0 else 128,y+16),(206 if dx>0 else 70,y+16)],K,3)
        p([(116,186),(162,186),(170,196),(108,196)],K)
    elif name=='КРОН':     # alarm clock
        el((86,46,194,154),ST2); el((92,50,190,148),IV); p([(140,50),(190,99),(140,148)],(206,198,178))
        el((70,30,106,66),ST); el((174,30,210,66),ST); ln([(104,156),(92,184)],K,6); ln([(176,156),(188,184)],K,6)
        ln([(141,99),(141,66)],K,6); ln([(141,99),(166,110)],K,4); el((135,93,147,105),R1)
        for k in range(12):
            a=k/12*2*math.pi; ln([(141+40*math.sin(a),99-40*math.cos(a)),(141+46*math.sin(a),99-46*math.cos(a))],K,3)
    elif name=='ВНЕШНИЙ':  # candlestick telephone: an outside call
        B1,B2=GOLD,GOLD2
        el((100,164,180,196),B2); el((104,162,176,188),B1); p([(132,176),(148,176),(146,80),(134,80)],B1); p([(140,176),(148,176),(146,80),(140,80)],B2)
        el((116,56,164,92),B1); el((140,60,164,92),B2); el((124,62,156,86),K)
        p([(168,72),(212,50),(224,62),(182,86)],(30,30,36)); el((204,40,238,74),(30,30,36)); el((210,46,232,68),(70,70,80)); ln([(150,176),(204,186),(242,166)],K,3)
        ln([(70,60),(90,74)],IV,4); ln([(60,90),(86,94)],IV,4); ln([(68,122),(90,114)],IV,4)
    elif name=='АВТОРИЗАЦИЯ':  # detective badge
        pts=[(140+70*math.cos(math.radians(-90+k*36))*(1 if k%2==0 else .62),112+70*math.sin(math.radians(-90+k*36))*(1 if k%2==0 else .62)) for k in range(10)]
        p(pts,GOLD); p([pts[0],pts[1],pts[2],pts[3],pts[4],pts[5],(140,112)],GOLD2); el((112,84,168,140),GOLD2); el((120,92,160,132),GOLD); tx((140,112),'★',26,K)
    elif name=='ЛОГИ':     # one drawer pulled out with cards
        r(50,40,230,190,WD2); r(58,48,222,110,WD); r(110,62,170,78,IV); r(126,90,154,98,GOLD)
        p([(40,126),(240,126),(252,190),(28,190)],WD); 
        for k in range(7): p([(56+k*2,118+k*9),(224-k*2,118+k*9),(224-k*2,124+k*9),(56+k*2,124+k*9)],IV if k%2 else (190,184,168))
        r(120,122,160,130,R1)
    elif name=='МОНИТОР':  # searchlight
        p([(70,150),(110,90),(150,110),(110,170)],ST2); el((96,80,166,150),ST); el((106,90,156,140),(255,244,210))
        p([(150,96),(260,20),(260,100),(162,128)],(110,120,130)); p([(110,170),(150,190),(90,190)],K)
    else:
        HB.icon(name,p,r,ln,el,tx)
ICONS=['КЛИЕНТЫ','ШЛЮЗ','КЭШ','ВОРКЕР','ОЧЕРЕДЬ','ПУЛ','БАЗА','БАЛАНСЕР','КРОН','ВНЕШНИЙ','АВТОРИЗАЦИЯ','ЛОГИ','МОНИТОР']
def icon_png(name):
    im,p,r,ln,el,tx=HB.sub(HB.CW,HB.CH,IV); icon_extra(name,p,r,ln,el,tx)
    return im.resize((HB.CW,HB.CH),Image.LANCZOS).crop((20,20,260,200))

# ---------------------------------------------------------------- parts
def export():
    shutil.rmtree(KIT,ignore_errors=True); os.makedirs(KIT+'/icons'); os.makedirs('pkg/fonts',exist_ok=True)
    shutil.copy(FONT,'pkg/fonts/Inter-ExtraBold.otf')
    g=lambda im:noirize(im,.22)
    # cork without anything on it
    b=HB.board(n_links=0,circled=False,notes=False)   # has title + cards; rebuild empty cork instead
    im=Image.new('RGBA',(W*SS,H*SS),(150,108,66,255)); d=ImageDraw.Draw(im)
    def p(pts,col): d.polygon([(x*SS,y*SS) for x,y in pts],fill=col+(255,))
    p([(0,0),(W,0),(W,300),(0,900)],(176,130,82)); p([(0,1500),(W,900),(W,H),(0,H)],(112,78,48)); p([(400,H),(W,1500),(W,H)],(84,56,34))
    rr=random.Random(5)
    for _ in range(900):
        x,y=rr.uniform(30,W-30),rr.uniform(30,H-30); s_=rr.uniform(2,5); p([(x,y),(x+s_,y+1),(x+s_-1,y+s_),(x-1,y+s_-1)],rr.choice([(120,84,50),(196,150,98),(96,66,40)]))
    for box in ((0,0,W,30),(0,H-30,W,H),(0,0,30,H),(W-30,0,W,H)): d.rectangle([v*SS for v in box],fill=HB.WOOD3+(255,))
    p([(30,30),(W-30,30),(W-30,44),(30,44)],(34,22,14)); p([(30,30),(44,30),(44,H-30),(30,H-30)],(34,22,14))
    g(im.convert('RGB').resize((W,H),Image.LANCZOS)).save(f'{KIT}/cork.png')
    # blank card: paper + empty photo slot
    c,cp,cr_,cl,ce,ct=HB.sub(HB.CW,HB.CH,IV); cr_(20,20,260,200,(24,28,36)); g(c.resize((HB.CW,HB.CH),Image.LANCZOS)).save(f'{KIT}/card_blank.png')
    for n in ICONS: g(icon_png(n)).save(f'{KIT}/icons/{n}.png')
    # title, notes
    t,tp,tr_,tl,te,tt=HB.sub(600,120,IV); tr_(0,0,16,120,R1); g(t.resize((600,120),Image.LANCZOS)).save(f'{KIT}/title_blank.png')
    for nm,col in (('note_yellow',(236,206,110)),('note_white',(226,222,208))):
        nt,np_,nr,nl,ne,ntx=HB.sub(244,132,col); np_([(0,132),(244,132),(244,96)],tuple(int(v*.82) for v in col)); g(nt.resize((244,132),Image.LANCZOS)).save(f'{KIT}/{nm}.png')
    # pins + arrow tags (arrow points right; rotate to the yarn direction)
    for nm,col in (('pin_gold',GOLD),('pin_red',R1)):
        im=Image.new('RGBA',(40*SS,44*SS),(0,0,0,0)); d=ImageDraw.Draw(im)
        e=lambda b,c_: d.ellipse([v*SS for v in b],fill=c_+(255,))
        e((8,12,40,42),(40,24,14)); e((1,1,31,31),tuple(int(v*.55) for v in col)); e((1,1,27,27),col); e((7,6,14,13),(255,236,220))
        g(im.resize((40,44),Image.LANCZOS)).save(f'{KIT}/{nm}.png')
    for nm,col in (('arrow_twine',IV),('arrow_red',R1)):
        im=Image.new('RGBA',(64*SS,56*SS),(0,0,0,0)); d=ImageDraw.Draw(im); sh=[(24,0),(-18,-20),(-8,0),(-18,20)]
        P=[((x+28)*SS,(y+24)*SS) for x,y in sh]; d.polygon([(x+7*SS,y+9*SS) for x,y in P],fill=(40,24,14,255)); d.polygon(P,fill=col+(255,)); d.line(P+[P[0]],fill=K+(255,),width=int(2.5*SS))
        g(im.resize((64,56),Image.LANCZOS)).save(f'{KIT}/{nm}.png')
    # marker circle (scale to fit the card + 50 px)
    im=Image.new('RGBA',(400*SS,420*SS),(0,0,0,0)); d=ImageDraw.Draw(im)
    for k,(dx,dy,a0,a1) in enumerate(((0,0,-70,292),(8,-6,-88,240))): d.arc(((6+dx)*SS,(8+dy)*SS,(394+dx)*SS,(412+dy)*SS),a0,a1,fill=R1+(255,),width=(9-k*3)*SS)
    g(im.resize((400,420),Image.LANCZOS)).save(f'{KIT}/marker_circle.png')

# ---------------------------------------------------------------- reference renderer (uses only kit files)
YARN={'twine':{'base':[214,188,134],'hi':[248,234,196],'lo':[124,98,60]},'red':{'base':[196,34,42],'hi':[246,110,104],'lo':[104,12,20]},
      'shadow':{'color':[40,24,14],'dx':9,'dy':13,'width':9},'outer':11,'core':8,'twist_every':6,'twist_len':5,'twist_w':2.2,
      'sag':'horizontal-ish (|dx| >= 0.3|dy|): +sin(pi*t)*(16+0.05*L) downward; steep: +sin(pi*t)*12 sideways','arrow_at':0.55}
def yarn_points(a,b):
    (x0,y0),(x1,y1)=a,b; L=math.hypot(x1-x0,y1-y0); n=max(2,int(L/3)); steep=abs(x1-x0)<abs(y1-y0)*.3; sag=16+L*.05
    return [(x0+(x1-x0)*t/n+(12*math.sin(math.pi*t/n) if steep else 0),y0+(y1-y0)*t/n+(0 if steep else sag*math.sin(math.pi*t/n))) for t in range(n+1)]
def pin_point(c,o):
    (x,y),(tx_,ty)=c['centre'],o['centre']; dx,dy=tx_-x,ty-y
    if abs(dx)<10: return (x+HB.CW/2-30,y+(HB.CH/2-28)*(1 if dy>0 else -1))
    hw,hh=HB.CW/2-24,HB.CH/2-24; s=min(hw/abs(dx),hh/abs(dy) if dy else 1e9); return (x+dx*s,y+dy*s)
def render_case(case,kit=KIT,steps=None):
    """case: {title,sub,cards:[{id,icon,label,sub,centre,rot,culprit}],links:[[from,to,red]],notes:[{t1,t2,colour,centre,rot}],culprit}
    steps: how many links are shown (None = all), then marker, then notes"""
    F=lambda s:ImageFont.truetype(f'pkg/fonts/Inter-ExtraBold.otf',s)
    K_=lambda n:Image.open(f'{kit}/{n}').convert('RGBA')
    im=K_('cork.png'); d=ImageDraw.Draw(im)
    def place(sub,cx,cy,rot,sh=(12,16)):
        r_=sub.rotate(rot,resample=Image.BICUBIC,expand=True); x,y=int(cx-r_.width/2),int(cy-r_.height/2)
        s_=Image.new('RGBA',r_.size,(30,18,10,255)); s_.putalpha(r_.getchannel('A')); im.alpha_composite(s_,(x+sh[0],y+sh[1])); im.alpha_composite(r_,(x,y))
    t=K_('title_blank.png'); td=ImageDraw.Draw(t); td.text((308,46),case['title'],font=F(58),fill=K,anchor='mm'); td.text((308,94),case['sub'],font=F(26),fill=(200,36,44),anchor='mm'); place(t,540,140,-1)
    byid={c['id']:c for c in case['cards']}
    for c in case['cards']:
        cd=K_('card_blank.png'); cd.alpha_composite(K_(f"icons/{c['icon']}.png"),(20,20)); dd=ImageDraw.Draw(cd)
        dd.text((HB.CW/2,246),c['label'],font=F(40 if len(c['label'])<7 else 34 if len(c['label'])<10 else 26),fill=(200,36,44) if c['id']==case.get('culprit') else K,anchor='mm')
        dd.text((HB.CW/2,284),c['sub'],font=F(20),fill=(110,104,96),anchor='mm'); place(cd,*c['centre'],c['rot'])
    links=case['links'] if steps is None else case['links'][:steps]; pins=[]
    for a,b,red in links:
        P0,P1=pin_point(byid[a],byid[b]),pin_point(byid[b],byid[a]); pts=yarn_points(P0,P1); Y=YARN['red' if red else 'twine']
        d.line([(x+9,y+13) for x,y in pts],fill=(40,24,14),width=9,joint='curve'); d.line(pts,fill=tuple(Y['lo']),width=11,joint='curve'); d.line(pts,fill=tuple(Y['base']),width=8,joint='curve')
        for i in range(0,len(pts)-2,2):
            (ax,ay),(bx_,by_)=pts[i],pts[i+1]; tx_,ty=bx_-ax,by_-ay; l=math.hypot(tx_,ty) or 1; tx_,ty=tx_/l,ty/l; nx,ny=-ty,tx_
            d.line([(ax-nx*4-tx_*2.5,ay-ny*4-ty*2.5),(ax+nx*4+tx_*2.5,ay+ny*4+ty*2.5)],fill=tuple(Y['hi'] if i%4==0 else Y['lo']),width=2)
        i=int(len(pts)*.55); ang=math.degrees(math.atan2(pts[i+3][1]-pts[i-3][1],pts[i+3][0]-pts[i-3][0]))
        ar=K_('arrow_red.png' if red else 'arrow_twine.png').rotate(-ang,resample=Image.BICUBIC,expand=True); im.alpha_composite(ar,(int(pts[i][0]-ar.width/2),int(pts[i][1]-ar.height/2)))
        pins+=[(P0,red),(P1,red)]
    for (x,y),red in pins: pn=K_('pin_red.png' if red else 'pin_gold.png'); im.alpha_composite(pn,(int(x-15),int(y-15)))
    full=steps is None or steps>len(case['links'])
    if full and case.get('culprit'):
        c=byid[case['culprit']]; m=K_('marker_circle.png'); im.alpha_composite(m,(int(c['centre'][0]-200),int(c['centre'][1]-210)))
    if steps is None or steps>len(case['links'])+1:
        for n in case.get('notes',[]):
            nt=K_(f"note_{n.get('colour','yellow')}.png"); nd=ImageDraw.Draw(nt); nd.text((122,46),n['t1'],font=F(38),fill=K,anchor='mm'); nd.text((122,94),n['t2'],font=F(30),fill=(200,36,44),anchor='mm')
            place(nt,*n['centre'],n.get('rot',0),(9,12)); pn=K_('pin_red.png'); im.alpha_composite(pn,(int(n['centre'][0]-15),int(n['centre'][1]-67)))
    return im.convert('RGB')

CASE_502={'title':'ДЕЛО № 502','sub':'12:03 · все запросы падают','culprit':'pool',
 'cards':[{'id':k,'icon':n,'label':n,'sub':HB.SUB[n],'centre':list(HB.POS[n]),'rot':HB.ROT[n]} for k,n in (('clients','КЛИЕНТЫ'),('gw','ШЛЮЗ'),('cache','КЭШ'),('worker','ВОРКЕР'),('queue','ОЧЕРЕДЬ'),('pool','ПУЛ'),('db','БАЗА'))],
 'links':[['clients','gw',0],['gw','cache',0],['gw','worker',1],['queue','worker',0],['worker','pool',1],['pool','db',0]],
 'notes':[{'t1':'ТАЙМАУТ','t2':'30 сек → 502','colour':'yellow','centre':[880,320],'rot':4},{'t1':'ВОРКЕРЫ','t2':'ждут ключ','colour':'white','centre':[186,1030],'rot':-4},{'t1':'КЛЮЧЕЙ','t2':'0 из 10','colour':'yellow','centre':[826,1770],'rot':3}]}
CASE_429={'title':'ДЕЛО № 429','sub':'02:00 · внешний сервис душит нас','culprit':'cron',
 'cards':[{'id':'cron','icon':'КРОН','label':'КРОН','sub':'задача в 02:00','centre':[290,420],'rot':-2},
          {'id':'lb','icon':'БАЛАНСЕР','label':'БАЛАНСЕР','sub':'раздаёт','centre':[790,560],'rot':2},
          {'id':'w1','icon':'ВОРКЕР','label':'ВОРКЕР 1','sub':'обработчик','centre':[290,900],'rot':-3},
          {'id':'w2','icon':'ВОРКЕР','label':'ВОРКЕР 2','sub':'обработчик','centre':[790,1020],'rot':2},
          {'id':'ext','icon':'ВНЕШНИЙ','label':'ВНЕШНИЙ API','sub':'лимит 100/с','centre':[290,1400],'rot':-1},
          {'id':'mon','icon':'МОНИТОР','label':'МОНИТОР','sub':'алерты','centre':[790,1720],'rot':3}],
 'links':[['cron','lb',1],['lb','w1',1],['lb','w2',1],['w1','ext',1],['w2','ext',1],['ext','mon',0]],
 'notes':[{'t1':'10 000','t2':'задач разом','colour':'yellow','centre':[810,1300],'rot':-4},{'t1':'ОТВЕТ','t2':'429 · ждите','colour':'white','centre':[290,1730],'rot':3}]}

if __name__=='__main__':
    export()
    json.dump(CASE_502,open('pkg/board_kit/case_502.json','w'),ensure_ascii=False,indent=1); json.dump(CASE_429,open('pkg/board_kit/case_429.json','w'),ensure_ascii=False,indent=1)
    json.dump({'yarn':YARN,'card':{'size':[HB.CW,HB.CH],'icon_at':[20,20],'label':{'at':[140,246],'px':'40 (≤6 chars) / 34 (≤9) / 26','colour':'ink; culprit red'},'sub':{'at':[140,284],'px':20,'colour':[110,104,96]},'shadow':[12,16],'shadow_colour':[30,18,10]},
               'title':{'at':[540,140],'rot':-1,'t1':{'at':[308,46],'px':58},'t2':{'at':[308,94],'px':26,'colour':'red'}},
               'note':{'t1':{'at':[122,46],'px':38},'t2':{'at':[122,94],'px':30,'colour':'red'},'pin':'pin_red centred 52 px above note centre','shadow':[9,12]},
               'pin':{'size':[40,44],'anchor':[15,15],'gold':'normal link','red':'failure link'},'arrow':{'anchor':'centre','points':'right; rotate to the yarn direction at 55% of its length'},
               'marker':{'file':'marker_circle.png','anchor':'centre on the culprit card, offset (-200,-210)'},
               'pin_point':'same column (|dx|<10): card corner (x+110, y±127); otherwise ray from card centre to the other centre, clipped to the card box shrunk by 24 px'},
              open('pkg/board_kit/spec.json','w'),ensure_ascii=False,indent=1)
    shutil.copy('board_kit.py','pkg/board_kit/reference_renderer.py')
    a=render_case(CASE_502); b=render_case(CASE_429); a.save('kit_502.png'); b.save('kit_429.png')
    G=Image.new('RGB',(1090,960)); G.paste(a.resize((540,960)),(0,0)); G.paste(b.resize((540,960)),(550,0)); G.save('kit_compare.png')
