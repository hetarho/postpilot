# Postpilot

사진과 "무슨 일이 있었는지"를 넣으면, **플랫폼에 그대로 붙여넣을 수 있는 블로그 글**을 만들어 주는 서비스.

이름은 `post` + `autopilot` — 포스팅의 자동조종 장치.

## 무엇을 만드나

1. 사진 여러 장을 올리고, 그날의 경험을 짧게 적는다.
2. 사진과 메모를 재료로 블로그 글 초안이 나온다.
3. 결과는 **플랫폼별 탭**으로 나뉘어 보인다. 탭을 열면 그 플랫폼에 바로 붙여넣을 수 있는 형태다.
   - **티스토리** — 붙여넣으면 그대로 완성되는 본문
   - **네이버 블로그** — 사진을 에디터에서 따로 올려야 하므로, 사진이 들어갈 자리를
     `[사진 1 — 설명]` 같은 **placeholder**로 남겨 둔 본문
   - 플랫폼은 이후 추가

> **현재 상태: 프레임워크 스캐폴드.** 위 로직은 아직 하나도 구현돼 있지 않다.
> 지금 있는 것은 배포까지 이어지는 뼈대 + 그 뼈대가 살아 있음을 증명하는 최소 왕복
> (백엔드 `HealthService.Ping`, 프론트 "Hello, world")뿐이다.
>
> **무엇을 어떻게 만들지는 [`PRD.md`](./PRD.md)가 권위다** — 기능 요구사항, 데이터 모델,
> 확정된 기술 선택과 그 근거가 거기 있다. 남은 작업 순서는 `PRD.md §10`.

## 스택

| 레이어 | 선택 | 배포처 |
|---|---|---|
| 프론트 | Vite + React 19 + TypeScript, Tailwind v4, TanStack Router/Query, FSD 구조 | Cloudflare Workers (정적 자산, `wrangler.jsonc`) |
| 전송 | Protobuf + **Connect** (buf 코드생성 → Go 핸들러 · TS 클라이언트), unary only | — |
| 백엔드 | Go + connect-go, distroless 정적 이미지 | GHCR 이미지 → VPS docker compose (공유 Caddy 뒤) |
| CI/CD | GitHub Actions (`ci.yml`, `deploy-backend.yml`) | `develop` → staging, `main` → prod |

DB·인증·저장소는 아직 **코드에 없다.** 무엇을 붙일지는 정해져 있다 —
SQLite(순수 Go 드라이버) · argon2id + HttpOnly 쿠키 세션 · 사진은 Cloudflare R2.
근거와 대안 비교는 `PRD.md §6`, 붙이는 순서는 `PRD.md §10`.

## 로컬 실행

```bash
cp .env.example .env
pnpm install

pnpm dev        # 프론트(:2564) + 백엔드(:7678) 동시
# 또는 따로
pnpm dev:web    # Vite  — http://localhost:2564
pnpm dev:api    # Go API in Docker (air 핫리로드) — http://localhost:7678/health
```

브라우저에서 `http://localhost:2564` 를 열면 "Hello, world" 아래에
`api: pong (v0.0.1)` 이 떠야 한다. 그게 프론트 → Connect → Go 왕복이 살아 있다는 뜻이다.

Docker 없이 백엔드만 띄우려면 (`.env`의 `PORT`를 따른다):

```bash
cd backend && go run ./cmd/api
```

> 개발 포트는 전화 키패드로 프로젝트를 읽은 것이다 — **2564 = B-L-O-G**(웹),
> **7678 = P-O-S-T**(api). 흔한 개발 기본값과도, 같은 머신의 cosimosi 스택(8080/1214)과도
> 겹치지 않는다. 컨테이너·운영 스택 안에서는 api가 평소대로 8080을 쓴다(compose가 7678→8080으로 매핑).

## 코드 생성

전송 계약은 `proto/postpilot/v1/*.proto` 하나가 진실이다. 고치면 재생성한다.

```bash
pnpm gen:proto   # buf(Docker) → backend/internal/gen + frontend/src/shared/api/gen
```

생성물은 커밋한다(빌드가 buf에 의존하지 않게). 손으로 고치지 않는다.

## 리포 구조

```
proto/postpilot/v1/     전송 계약 (단일 진실)
backend/
  cmd/api/              합성 루트 — 설정·서버 조립만
  internal/health/      HealthService 구현 (지금은 이것뿐)
  internal/platform/    config, rpcserver (mux·h2c·CORS·/health)
  internal/gen/         buf 생성물 (수정 금지)
frontend/src/           FSD: app / pages / widgets / features / entities / shared
deploy/edge/            VPS 공유 Caddy (80/443 단일 소유자)
scripts/                codegen 래퍼 (Docker 경유)
```

## 규칙

- **로직은 `internal/<도메인>`, 조립은 `cmd/api`.** `rpcserver`에 비즈니스 로직을 넣지 않는다.
- **프론트는 FSD.** 슬라이스는 `index.ts`로만 노출한다 (`pnpm --filter ./frontend lint:fsd`).
- **순수 레이어(`shared/api`, `shared/config`, `shared/lib`, `entities/*/model`)는
  react/react-dom을 import하지 않는다.** ESLint boundaries가 막는다.
- **`VITE_*`는 빌드 타임에 번들에 박힌다** — 공개 값만. 비밀은 백엔드 env로.
