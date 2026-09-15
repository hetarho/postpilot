# 초록 배경 디자인 미리보기 생성기

도커 없이 클립 렌더 결과를 정적으로 보기 위한 임시 도구. 실제 렌더 경로
(`layoutComposition` → `declaredSVG`)가 내보내는 SVG 그대로이고, 글자 폭 측정만
resvg 대신 번들 폰트(Pretendard Variable, Paperlogy)의 글리프 메트릭으로 대신한다.
배경은 스모크 테스트와 같은 단색 초록 #1F7A3D이고, 그 휘도를 `Luminance`로 주입해
스크림·액센트 판정도 실제와 같은 경로를 탄다.

## 1. SVG 생성

```sh
cp tmp/gallery/generator.go.txt backend/internal/clip/media/zz_gallery_preview_test.go
cd backend && CLIP_PREVIEW_GALLERY=1 go test ./internal/clip/media/ -run TestWriteGreenGallery -count=1 -v
rm backend/internal/clip/media/zz_gallery_preview_test.go
```

`tmp/gallery/`에 비율·시각별 SVG와 `index.html`이 생성된다. 실행 로그에는
중앙 정렬 요소의 잉크 중심(`CENTRE`)과 슬롯 겹침(`OVERLAP`)이 함께 출력된다.

겹침이 하나라도 발견되면 테스트는 `FAIL`로 끝나지만 파일은 정상 생성된다. 지금은
1:1 아웃트로 E가 걸리며, T176이 고칠 대상이다.

장면을 바꾸려면 `generator.go.txt`의 `scenes` 목록에서 composition XML과
프레임 시각(ms)을 고친다.

## 2. PNG 렌더 (선택)

브라우저로 `index.html`을 열면 되지만, PNG가 필요하면 resvg로 굽는다.

resvg는 이 저장소의 의존성이 아니다. pnpm 워크스페이스를 건드리지 않도록 저장소 밖에
설치하고 거기서 실행한다.

```sh
mkdir -p ~/.cache/clip-preview && cd ~/.cache/clip-preview
npm i @resvg/resvg-js
node <repo>/tmp/gallery/render.mjs
```

`render.mjs`는 자기 위치에서 경로를 풀기 때문에 어디서 실행해도 되고, 시스템 폰트 없이
번들 TTF 두 개만 넘겨 실제 파이프라인과 같은 폰트로 그린다. `index.html`은 PNG를
참조하므로 이 단계를 거쳐야 그림이 보인다.

## 주의

생성기는 소스 트리에 남기지 않는다. media 패키지 내부 헬퍼(`declaredPlan`,
`measuredDeclared`, `testRenderer`)를 쓰기 때문에 패키지 안에 있어야 컴파일되지만,
테스트 스위트의 일부가 아니다.
