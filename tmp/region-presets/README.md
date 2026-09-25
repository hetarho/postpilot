# 인트로·아웃트로 프리셋 시안 + 제목 폭 맞춤 실험

글자 수 고정(CDS-20·CDS-77) 대신 **856px 글줄 폭을 넘지 않는 선에서 글자 크기를 줄이는**
규칙을 여러 방식으로 그려 보고, 인트로·아웃트로 프리셋을 10종씩 새로 그려 본 정지 이미지 시안.
영상은 만들지 않았다.

- 폰트: 저장소 번들(`backend/assets/fonts`) 그대로, `--skip-system-fonts`
- 측정: 프로덕션과 같은 `resvg --query-all` advance 폭 (100px에서 재고 선형 비례)
- 배경: `tmp/footage/vertical.mp4`의 3.0s(접시)·11.9s(불판) 프레임, 광고 배지는 delogo로 지움
- 색·외곽선·그림자·가로선·바는 `design.json` 토큰 값

## 결과물 (`out/`)

모든 예시는 9:16 세로 한 화면(1080×1920)이다.

| 파일 | 내용 |
|---|---|
| `page.html` | 게시용 페이지 — 비교 격자, 시안 카드(짧은 문구 / 긴 문구 / 슬롯 구조), 채택 복사 |
| `web/full/*.jpg` · `web/thumb/*.jpg` | 페이지가 쓰는 원본·축소 이미지 (`img/full/`·`img/thumb/`로 게시) |
| `01-fit-size-rules.jpg` | 크기를 정하는 방식 6가지 × 문구 길이 4단계 (인트로 A) |
| `02-fit-vertical-rules.jpg` | 줄어들 때 세로 배치 — 기준선 고정 vs 간격 유지 (아웃트로 E) |

## 프리셋은 크기와 위치만 정한다

프리셋은 슬롯 목록(역할·글자 크기·위치)과 선·바·테두리·판·스크림 같은 중립 장식만 갖는다.
화면의 글자는 전부 템플릿 문구가 슬롯 순서대로 한 줄씩 채우고, 모자라면 그 슬롯은 비운다.
고정 문구(따옴표·접두·단위), 문구 이어 붙이기, 뜻이 담긴 아이콘(별·핀·체크·책갈피·화살표)은 없다.
`presets.py`의 `INTROS`/`OUTROS`가 슬롯 정의이고, 예시 문구는 `content.json`에서 `keys` 순서로 꺼낸다.

## 제안 규칙 (⑤ + 간격 유지)

1. 기본 크기에서 한 줄 폭을 잰다. 856px 안이면 그대로.
2. 넘치면 `기본 × 856 / 폭`으로 연속 축소, 하한까지.
   하한: display 80 · headline 60 · hook 56 · title 52 · body 48 · caption 40 · label 34
   (작은 글자는 CDS-3 최소 크기 그대로)
3. 하한 아래로 가야 하면 큰 글자(display·headline·hook·title)만 어절 경계에서 두 줄로 나누고
   다시 1–2를 적용한다. 작은 글자는 한 줄.
4. 그래도 안 들어가면 지금처럼 — 작성 문구는 저장 전에 짚고, 생성 문구는 짧게 고친다.
5. 슬롯 사이는 잉크 간 간격을 고정하고 블록 중심만 지킨다(기준선 고정이 아님).

한 줄에 들어가는 한글 글자 수(공백 제외)는 이렇게 바뀐다.

| 역할 | 지금 한도 | 기본 크기에서 실제로 들어가는 수 | 하한 크기에서 |
|---|---|---|---|
| display 132 | 6 | 7 | 11 (두 줄 22) |
| headline 96 | 8 | 10 | 16 (두 줄 32) |
| hook 84 | 9 | 11 | 17 (두 줄 34) |
| title 72 | 11 | 13 | 19 (두 줄 38) |
| body 56 | 14 | 17 | 20 |
| caption 44 | 18 | 22 | 24 |
| label 36 | 22 | 26 | 28 |

## 다시 그리기

```sh
cd tmp/region-presets/src
python3 fit_sheet.py      # out/cells/*.png, fit.json, 01·02
python3 presets.py        # out/frames/*-{short,long,slots}.png, frames.json
cd ../out && for f in cells/*.png frames/*.png; do b=$(basename $f .png)
  ffmpeg -v error -y -i $f -q:v 4 web/full/$b.jpg
  ffmpeg -v error -y -i $f -vf scale=480:-1 -q:v 5 web/thumb/$b.jpg; done
cd ../src && python3 build_page.py   # out/page.html
```

필요 도구는 Python 3.9 표준 라이브러리, `resvg` 0.48.1, `ffmpeg`.
