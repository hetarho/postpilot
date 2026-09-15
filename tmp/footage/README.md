# 실제 촬영본 위 디자인 미리보기 생성기

`tmp/gallery`가 단색 초록 위에 SVG만 그려 보여준다면, 이쪽은 **실제 렌더 경로를
끝까지 태워서 mp4를 만든다**. 오너가 찍은 원본(현재는 한우 구이집 촬영본)을
소스로 써서 컷·전환·자막 페이드·밝기 샘플링·스크림 판정이 전부 실제와 같은 코드로
돌아가고, 결과는 브라우저나 QuickTime에서 그냥 재생해 보면 된다.

도커 대신 호스트의 ffmpeg / ffprobe / resvg를 쓴다. 이미지의 ffmpeg는
`--disable-everything` 빌드라 필터 허용목록이 따로 있지만(→ media-tools.sh),
호스트 ffmpeg는 전부 들어 있으므로 **이 생성기가 통과했다고 도커에서 통과하는
것은 아니다**. 릴리스 게이트는 여전히 `CLIP_MEDIA_SMOKE=1` 도커 실행이다.

## 준비

```sh
brew install resvg   # 0.48.1에서 확인
```

ffmpeg / ffprobe는 `/opt/homebrew/bin` 기본값을 쓰고, 다른 경로면
`CLIP_FFMPEG_PATH` · `CLIP_FFPROBE_PATH` · `CLIP_RESVG_PATH`로 넘긴다.
폰트는 저장소에 번들된 Pretendard Variable + Paperlogy를 그대로 쓴다.

## 실행

```sh
cp tmp/footage/generator.go.txt backend/internal/clip/media/zz_footage_preview_test.go
cd backend && CLIP_FOOTAGE_PREVIEW=1 \
  CLIP_FOOTAGE_DIR="/path/to/originals" \
  go test ./internal/clip/media/ -run TestRenderFootagePreview -count=1 -v -timeout 60m
rm backend/internal/clip/media/zz_footage_preview_test.go
```

`tmp/footage/`에 `vertical.mp4` · `vertical-b.mp4` · `square.mp4` ·
`horizontal.mp4`가 떨어진다. 같은 스토리보드·같은 문구를 인트로/아웃트로 프리셋과
비율만 바꿔 렌더한 것이라, 네 개를 나란히 놓고 보면 프리셋 선택이 화면에서
어떻게 다른지 바로 보인다.

- `vertical` — 인트로 a(헤드라인+라벨) · 아웃트로 e(라벨+디스플레이+캡션+바)
- `vertical-b` — 인트로 b(훅+헤어라인 두 줄) · 아웃트로 b(훅+바디)
- `square` / `horizontal` — CDS-46~48의 비율별 위치 재정의 확인용

실행 로그에는 렌더된 요소가 역할·위치·노출구간·좌표와 함께 찍히고, 마지막에
`design.VerifyRenderable`을 한 번 더 돌린다.

## 스토리보드 바꾸기

`previewShots`가 컷 목록(원본 파일명·소스 구간·전환), `previewBody`가 템플릿
XML이다. 컷은 `SectionID`로 XML의 `<scene>`과 묶이므로 둘을 같이 고쳐야 한다.
`plan.DurationMS`는 컷 길이 합에서 전환 겹침을 뺀 값으로 자동 계산된다.

주의: 컷 자막(`basis="cut"`)의 노출은 CDS-41의 `900 + 90 × 한글자수` ms를 넘겨야
하고, 못 넘기면 렌더가 `readability`로 거절한다. 마지막 컷은 닫는 블록이 덮으므로
자막을 두지 않았다.

## 주의

생성기는 소스 트리에 남기지 않는다. 패키지 내부 헬퍼(`copyOriginal`, `New`,
`NewRenderer`)를 쓰기 때문에 패키지 안에 있어야 컴파일되지만, 테스트 스위트의
일부가 아니다.
