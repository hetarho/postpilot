# Postpilot

사진과 "무슨 일이 있었는지"를 넣으면, **플랫폼에 그대로 붙여넣을 수 있는 블로그 글**을 만들어 주는 서비스.

이름은 `post` + `autopilot` — 포스팅의 자동조종 장치.

## 무엇을 만드나

1. 사진 여러 장을 올리고, 그날의 경험을 짧게 적는다.
2. 사진과 메모를 재료로 블로그 글 초안이 나온다.
3. 결과는 **플랫폼별 탭**으로 나뉘어 보인다. 탭을 열면 그 플랫폼에 바로 붙여넣을 수 있는 형태다.
   - **티스토리** — 붙여넣으면 그대로 완성되는 본문
   - **네이버 블로그** — 사진을 에디터에서 따로 올려야 하므로, 사진이 들어갈 자리를
     `사진_1_설명_사진` 같은 **placeholder**로 남겨 둔 본문
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

DB와 인증은 들어와 있다 — SQLite(순수 Go 드라이버, WAL, 바이너리에 embed된 goose
마이그레이션이 기동 때 실행) + argon2id · HttpOnly 쿠키 세션. 사진 저장소(Cloudflare R2)는
아직 없다. 근거와 대안 비교는 `PRD.md §6`, 붙이는 순서는 `PRD.md §10`.

## 로컬 실행

Node 버전은 `.node-version`에 고정되어 있다. fnm·nvm·asdf 같은 도구는 이 파일을 읽어
자동으로 맞춰 주고, CI도 같은 파일을 읽는다. 도구가 없다면 그 버전을 직접 설치한다.

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

### 발행

자동 발행 기능은 폐기됐다. 확정한 글은 내보내기에서 플랫폼별 형식으로 복사하고, 사진과 영상은
사용자가 해당 플랫폼 편집기에 직접 옮긴다.

### 계정 만들기

가입 화면은 없다(PRD F-1). 계정은 운영자가 만든다:

```bash
cd backend && go run ./cmd/adduser <login_id>   # 비밀번호 2회 입력
```

컨테이너에서는 같은 코드가 api 바이너리의 서브커맨드로 붙어 있다 —
`docker compose run --rm backend go run ./cmd/api adduser <login_id>` (운영은 `DEPLOY.md §4`).
`/health`를 제외한 모든 RPC는 세션 쿠키가 없으면 401이다.

### 테스트 데이터 시드

화면을 보려면 계정마다 글이 몇 개씩은 있어야 한다. `--seed`는 로컬 설치의 계정 데이터를
지우고 고정된 테스트 계정 다섯 개로 바꾼다:

```bash
pnpm dev --seed                   # 시드 후 dev 서버 기동
pnpm dev --seed --purge-objects   # MinIO 버킷까지 비운다
```

| 로그인 | 플랜 | 글 | 초안 / 검토 / 확정 / 발행 |
|---|---|---|---|
| `free` | free | 0 | 0 / 0 / 0 / 0 |
| `base` | basic | 3 | 2 / 1 / 0 / 0 |
| `pro` | pro | 8 | 3 / 2 / 1 / 2 |
| `max` | max | 14 | 4 / 3 / 7 / 0 |
| `master` | master | 23 | 5 / 4 / 3 / 11 |

비밀번호는 다섯 계정 모두 `seed-only`이고, 이메일 없이 위 아이디로 로그인한다. 계정이 하나라도 비어 있어야 새 가입 직후의 화면을 확인할 수 있으므로 `free`는
일부러 글이 없다.

품질 화면도 네이버 키 없이 확인할 수 있다. `pro`와 `master`의 글에는 분야 일상·생각이 들어
있고, 생성된 글에는 모두 명사가 저장된다. `master`의 발행 글 11개는 모든 품질 지표의 최소
발행 수를 채우고, `pro`의 2개는 최소에 못 미친 줄을 보여 준다. 두 계정의 검토 글은 ②에서
교체 표시(제목·태그·본문)를 보여 주고, `master`의 첫 초안에는 제목 형식이 있는 템플릿
`하루 기록`이 지정돼 있다. 일상·생각 분야의 문구 목록은 그 분야에 목록이 없을 때만 쓰고,
어떤 시드도 지우지 않는다 — 네이버 키가 있는 서버에서 실제로 모은 목록도 그대로 남는다.

시드는 `users`를 지우고 스키마의 CASCADE에 나머지를 맡긴다. 따라서 글·말투·크레딧·clip
프로젝트는 함께 사라지지만, 설치 전체가 공유하는 큐레이션 — `/admin`에서 등록한 모델과
그 용도 — 은 남는다. 후보 목록은 OpenRouter가 다시 주지만 어떤 모델을 어떤 용도로 쓸지는
운영자의 선택이고, 그것까지 지우면 시드 직후에는 아무것도 생성할 수 없다.

무엇을 만드는지는 `backend/internal/devseed`가 정하고, 실행은 `backend/cmd/seed`다.
운영 이미지는 `./cmd/api`만 빌드하므로 이 명령은 배포된 바이너리에 존재하지 않는다 —
계정을 전부 지우는 명령이 운영 ENTRYPOINT에서 닿지 않는다는 뜻이다.

> 개발 포트는 전화 키패드로 프로젝트를 읽은 것이다 — **2564 = B-L-O-G**(웹),
> **7678 = P-O-S-T**(api). 흔한 개발 기본값과도, 같은 머신의 cosimosi 스택(8080/1214)과도
> 겹치지 않는다. 컨테이너·운영 스택 안에서는 api가 평소대로 8080을 쓴다(compose가 7678→8080으로 매핑).

## 코드 생성

전송 계약은 `proto/postpilot/v1/*.proto` 하나가 진실이다. 고치면 재생성한다.

```bash
pnpm gen:proto   # buf(Docker)  → backend/internal/gen + frontend/src/shared/api/gen
pnpm gen:sql     # sqlc(Docker) → backend/internal/<컨텍스트>/store/sqlc
pnpm gen         # 둘 다
```

sqlc가 읽는 스키마는 `backend/internal/platform/db/migrations/` — 기동 때 실제로 도는
그 파일들이다. 스키마를 바꾸면 마이그레이션을 추가하고 `pnpm gen:sql`을 다시 돌린다.

생성물은 커밋한다(빌드가 buf/sqlc에 의존하지 않게). 손으로 고치지 않는다.

## 이미지 게이트

clip의 media/render 게이트는 호스트에서 돌지 않는다 — 번들 ffmpeg·resvg·폰트가 들어 있는
프로덕션 이미지 안에서만 돈다. 그래서 검증할 때마다 이미지가 하나씩 남는다.

```bash
pnpm smoke           # production — media/input 스모크가 빌드 중에 강제로 돈다
pnpm smoke:media     # media-smoke 타깃만
pnpm smoke:input     # clip-input-smoke 타깃만
pnpm smoke:release   # release-smoke 빌드 + 실행 (--memory 1g --cpus 2 를 테스트가 검사한다)
pnpm docker:gc       # 남은 게이트 이미지 정리 (목록만; 지우려면 --yes)
```

**게이트 이미지는 위 고정 태그(`postpilot:gate*`)를 쓴다. 태스크마다 새 태그를 만들지
않는다.** 태그가 매번 다르면 이전 이미지가 계속 참조된 상태로 남아 `docker image prune`이
영원히 빈손으로 돌고, 실제로 그렇게 60개 18 GB가 쌓여 디스크가 찼다. 고정 태그면 새 빌드가
이전 것을 밀어내고, 밀려난 것은 `pnpm docker:gc`가 쓸어간다.

태스크 파일에 남기는 증거는 태그 이름이 아니라 **무엇이 통과했는지**다 — 이미지는 지워도 된다.

`pnpm docker:gc`는 실행 중이든 멈췄든 컨테이너가 참조하는 이미지, 최근 24시간 안에 빌드된
이미지(다른 세션이 쓰는 중일 수 있다), `postpilot*`/`pp-builder` 밖의 리포지토리는 건드리지
않는다. 빌드 캐시는 `--cache`를 줘야 비우는데, 비우면 다음 빌드가 ffmpeg/resvg 소스 빌드부터
다시 도니 디스크가 급할 때만 쓴다.

## 리포 구조

```
proto/postpilot/v1/     전송 계약 (단일 진실)
backend/
  cmd/api/              합성 루트 — 설정·마이그레이션·서버 조립만
  cmd/adduser/          운영자용 계정 생성 (api 서브커맨드와 같은 코드)
  cmd/seed/             로컬 테스트 데이터 시드 — 운영 이미지에 빌드되지 않는다
  internal/devseed/     시드가 만드는 계정·글 픽스처 (무엇을 만드는지의 단일 진실)
  internal/auth/        계정·세션 컨텍스트 (service · store · rpc · provision)
  internal/health/      HealthService 구현
  internal/platform/    config, db(SQLite·마이그레이션), rpcserver (mux·h2c·CORS·/health)
  internal/gen/         buf 생성물 (수정 금지)
frontend/src/           FSD: app / pages / widgets / features / entities / shared
deploy/edge/            VPS 공유 Caddy (80/443 단일 소유자)
scripts/                dev 런처(--seed) · codegen 래퍼 (Docker 경유) · docker-gc
```

## 규칙

- **도커 게이트 이미지는 고정 태그.** `postpilot:gate*`만 쓰고 태스크별 태그를 만들지 않는다 (`pnpm docker:gc`).
- **로직은 `internal/<도메인>`, 조립은 `cmd/api`.** `rpcserver`에 비즈니스 로직을 넣지 않는다.
- **프론트는 FSD.** 슬라이스는 `index.ts`로만 노출한다 (`pnpm --filter ./frontend lint:fsd`).
- **프론트 포맷은 Prettier가 결정한다** (`pnpm format:check`, `pnpm lint`와 CI에 포함). 고치려면
  `pnpm --filter ./frontend format`을 쓴다. `dist/`와 buf 생성물 `src/shared/api/gen/`은
  `frontend/.prettierignore`로 제외한다 — 생성물을 포맷하면 재생성 결과와 영구히 어긋난다.
- **순수 레이어(`shared/api`, `shared/config`, `shared/lib`, `entities/*/model`)는
  react/react-dom을 import하지 않는다.** ESLint boundaries가 막는다.
- **`VITE_*`는 빌드 타임에 번들에 박힌다** — 공개 값만. 비밀은 백엔드 env로.
- **세션 토큰은 쿠키에만 있다.** 응답 본문·로그·URL에 절대 넣지 않는다. 인증된 핸들러는
  행위 주체를 인터셉터가 넣어준 context에서 읽고, 요청 payload에서 읽지 않는다.
