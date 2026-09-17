import struct, os, json
def _tbl(d):
    num,=struct.unpack('>H',d[4:6]); t={}
    for i in range(num):
        e=12+16*i; t[d[e:e+4]]=struct.unpack('>I',d[e+8:e+12])[0]
    return t
def cmap_set(p):
    d=open(p,'rb').read(); off=_tbl(d)[b'cmap']
    n,=struct.unpack('>H',d[off+2:off+4]); best=None
    for i in range(n):
        r=off+4+8*i; pid,eid,so=struct.unpack('>HHI',d[r:r+8])
        if (pid,eid) in ((3,1),(3,10),(0,3),(0,4)): best=off+so
    fmt,=struct.unpack('>H',d[best:best+2]); s=set()
    if fmt==4:
        segx2,=struct.unpack('>H',d[best+6:best+8]); sc=segx2//2
        end=best+14; start=end+segx2+2
        for i in range(sc):
            e_,=struct.unpack('>H',d[end+2*i:end+2*i+2]); s_,=struct.unpack('>H',d[start+2*i:start+2*i+2])
            if s_==0xFFFF: continue
            s.update(range(s_, min(e_,0xFFFF)+1))
    elif fmt==12:
        ng,=struct.unpack('>I',d[best+12:best+16])
        for i in range(ng):
            g=best+16+12*i; a,b,_=struct.unpack('>III',d[g:g+12])
            s.update(range(a,min(b,0x2FFFF)+1))
    return s
def coverage(p):
    s=cmap_set(p)
    hangul=sum(1 for c in range(0xAC00,0xD7A4) if c in s)
    punct="·—’“”…，。！？％「」『』〈〉"
    ascii_ok=all(ord(c) in s for c in "ABCabc0123456789.,!?()%:'\"-")
    return {"glyphs":len(s),"hangul":hangul,"hangul_pct":round(hangul/11172*100,1),
            "punct_missing":"".join(c for c in punct if ord(c) not in s),"ascii_ok":ascii_ok}
