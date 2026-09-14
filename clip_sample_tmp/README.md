# clip_sample_tmp — 클립 오버레이 디자인 검토 샘플

오너가 직접 보고 남길/뺄 스타일을 고르기 위한 임시 자료. 디자인이 확정되고
`design.json` / CDS SSOT에 반영되면 이 디렉토리는 지운다.

## current/ — 지금 코드가 실제로 뽑는 화면

`style-gallery.mp4` (23초, 1080×1920)는 프로덕션 렌더 경로 그대로 돌린 결과다.
`TestStyleGalleryExample` (backend/internal/clip/media/style_gallery_test.go)을
Docker `media-smoke` 이미지 안에서 실행해 만들었다 — resvg + 번들 폰트 + ffmpeg,
호스트에는 이 도구들이 없다.

```
docker build --target media-smoke -t postpilot-style-gallery -f backend/Dockerfile .
docker run --rm --memory 2g --cpus 2 \
  -v "$PWD/clip_sample_tmp/current":/export/clips \
  -e CLIP_MEDIA_SMOKE=1 -e CLIP_STYLE_GALLERY=1 -e CLIP_SMOKE_EXPORT=/export/clips \
  --entrypoint /media.test postpilot-style-gallery \
  -test.run=TestStyleGalleryExample -test.v
```

컷 순서와 스틸:

| 스틸 | 컷 | 본 것 |
|---|---|---|
| `00-hook.png` | hook 카드 | 오너 판단: 제거 |
| `01-clean.png` | `clean` 깔끔하게 | 어두운 판 + 액센트 바. 오너 판단: 제거 |
| `02-memo.png` | `memo` 메모 | 밝은 종이판. 같은 컷의 정보 칩을 밀어냄(둘 다 top/left) |
| `03-bold.png` | `bold` 크게 강조 | 오너 판단: **이걸 자막 기본으로** |
| `04-mark.png` | `mark` 형광펜 | 하이라이트가 글자 사이 세로 조각으로 깨져 나옴 |
| `05-simple.png` | `simple` 가벼운 텍스트 | 초록 단색 위에서 가장 약함 |
| `06-rapid-a.png` `07-rapid-b.png` | rapid 분절 | 문장이 조각으로 끊겨 노출됨 |
| `08-ending.png` | ending 카드 | 오너 판단: 제거 |

## explore/ — 인트로·아웃트로 대안 시안

카드와 알약(rounded-full)을 쓰지 않고, 중앙정렬로, 크기·가로선·반투명만으로
구성한 시안. `gen.py`가 1080×1920 아트보드를 만들고 resvg가 래스터화한다.
실제 번들 폰트와 CDS 토큰(색·그림자·스트로크·액센트)을 그대로 쓴다.

```
python3 clip_sample_tmp/explore/gen.py clip_sample_tmp/explore
docker run --rm -v "$PWD/clip_sample_tmp/explore":/w \
  --entrypoint /usr/local/bin/resvg postpilot-style-gallery \
  --skip-system-fonts \
  --use-font-file /usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf \
  --use-font-file /usr/share/postpilot-fonts/paperlogy/Paperlogy-8ExtraBold.ttf \
  --font-family "Pretendard Variable" /w/intro-1.svg /w/intro-1.png
```

| 시트 | 시안 |
|---|---|
| `intro-1.png` | A 크기만 · B 위아래 가로선 · C 반투명 밴드 α0.35 |
| `intro-2.png` | D 반투명 박스 α0.55 · E accent 가로선 · F 흐린 라벨 + 선명 값 |
| `outro-1.png` | A 숫자 크게 · B 가로선 구분 · C 반투명 밴드 |
| `outro-2.png` | D 미니멀 · E 점수 + accent 선 · F 라벨 스택 |

## 여기서 확인된 버그 두 개

1. **Paperlogy가 한 번도 쓰인 적이 없다.** `design.json`의 `faces.paperlogy`가
   `"Paperlogy 8 ExtraBold"`인데, CSS font-family 문법상 숫자로 시작하는 구성요소는
   무효라 resvg가 파싱에 실패하고 Pretendard로 조용히 폴백한다
   (`Failed to parse font-family value ... Falling back to Pretendard Variable`).
   폰트 파일의 typographic family(nameID 16)는 `Paperlogy`이고, 이 이름만 매칭된다.
   hook·title·크게 강조 전부가 지금까지 Pretendard로 렌더돼 왔다.
   `explore/` 시안은 고친 이름(`Paperlogy`)으로 뽑았으므로 `current/`와 서체가 다르다.

2. **형광펜(`mark`) 하이라이트가 깨진다.** `04-mark.png`에서 `18,000원` 뒤의
   액센트 밑줄이 연속된 막대가 아니라 글자 사이 세로 조각으로 나온다.
   키워드에 쉼표가 들어갈 때 글자 advance 계산이 어긋나는 것으로 보인다.
