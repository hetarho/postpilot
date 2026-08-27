# 배포 — postpilot 운영 문서

**현재 아무것도 배포돼 있지 않다.** 이 문서는 리포가 이미 갖춘 배포 뼈대가
*무엇을 전제로 하는지*와, 그 환경을 **처음부터 세우는 절차**를 적는다.
시크릿 값은 절대 커밋하지 않는다 — 이름과 위치만 적는다(§3).

구조는 cosimosi와 동일하다. VPS를 이미 쓰고 있다면 **같은 박스·같은 edge Caddy를
그대로 재사용**하면 된다(§5의 3번 부트스트랩을 건너뛴다).

## 1. 목표 구조

```
브라우저
 ├─ postpilot.<도메인> ───────────────▶ Cloudflare Worker `postpilot` (정적 자산, main 빌드)
 └─ api.postpilot.<도메인> ──────────▶ VPS
                                        └─ edge Caddy(80/443, TLS 자동 발급)
                                            ├─ api.postpilot.<도메인>         → postpilot-api-prod:8080
                                            └─ api.staging.postpilot.<도메인> → postpilot-api-staging:8080
이미지: ghcr.io/hetarho/postpilot-api:<커밋 SHA>
```

| 항목 | 값 |
|---|---|
| 프론트 prod | Cloudflare Worker `postpilot` (`wrangler.jsonc`의 name과 반드시 일치) |
| 프론트 프리뷰 | `<버전8자리>-postpilot.<계정서브도메인>.workers.dev` (main 외 브랜치 push마다) |
| 백엔드 | VPS 1대, 스택 2개 — `/srv/postpilot-staging`, `/srv/postpilot-prod` |
| API 도메인 | prod / staging 각 1개 — DNS는 **회색 구름(DNS only)** 필수 |
| 브랜치 매핑 | 지금은 `main` → prod 하나만 쓴다. staging 스택과 워크플로의 분기는 그대로 살아 있어서, `develop` 브랜치를 만들고 `deploy-backend.yml`의 `branches`에 추가하면 바로 동작한다 |
| DB | SQLite, 스택 볼륨 안 (`/srv/postpilot-<env>/data`) |

## 2. 일상 배포 (자동)

**백엔드** — `main`에 백엔드 경로(`backend/**`, `docker-compose.prod.yml`, 워크플로)가 바뀐
push가 가면 `deploy-backend.yml`이 이 순서로 돈다.

```
build    Compose 검증 → 이미지 빌드 → GHCR push (:<sha> + :prod 두 태그)
rollout  compose 파일을 VPS로 동기화
         → .env 백업, API_UPSTREAM·CORS_ORIGIN 비어 있으면 손대기 전에 중단
         → pull (실패해도 다운타임 없음)
         → up -d  (SQLite는 단일 라이터라 컨테이너를 교체한다)
         → /health 게이트 (재시도 15회)  ✗ 이면 이전 IMAGE_TAG로 되돌리고 실패
verify   브라우저 origin으로 CORS preflight 확인 (credentials 포함)
```

- **마이그레이션 스텝이 따로 없는 건 의도다.** DB가 이 스택 볼륨 안의 SQLite 파일이라
  맞춰야 할 공용 서버가 없다. 마이그레이션은 API 바이너리에 embed되어 기동 시 돈다 —
  스키마가 그걸 읽는 코드와 어긋날 수 없다. 실패하면 프로세스가 죽고, `/health` 게이트가
  잡아내고, rollout이 이전 태그로 되돌린다.
- **CORS 검증이 rollout과 분리된 것도 의도다.** CORS 불일치는 이미지 문제가 아니라 스택
  `.env` 문제라 되돌려도 안 고쳐진다. 시끄럽게 실패해야 한다. 세션 쿠키가 정확한
  origin + credentials 허용에 의존하므로(PRD F-1), 이게 "로그인이 조용히 안 되는" 사고를
  잡는 검사다.
- 수동 재배포: Actions 탭 → Deploy backend → **Run workflow**.

`DEPLOY_ENABLED` variable이 없으면 **Compose 검증 + 이미지 빌드까지만** 하고 push·rollout을
건너뛴다(실패가 아니라 스킵) — VPS를 세우기 전에도 CI가 초록으로 유지된다.

**프론트** — Cloudflare Workers Builds(네이티브 Git 연동)가 push를 감지해 빌드한다.
`main` → `npx wrangler deploy`(프로덕션 승격), 그 외 브랜치 → `npx wrangler versions upload`
(프리뷰 버전만).

## 3. 키·시크릿 인벤토리 (값은 여기 없음 — 위치만)

| 이름 | 어디에 | 용도 |
|---|---|---|
| `SSH_HOST`/`SSH_USER`/`SSH_KEY` | GitHub repo secrets | Actions→VPS 배포 접속 (배포 전용 ed25519 개인키) |
| `DEPLOY_ENABLED=true` | GitHub repo **variable** | 배포 스위치 — 지우면 rollout이 건너뛰어짐(빌드 검증만) |
| `API_ORIGIN` | GitHub repo **variable** | 배포 후 `/health` 게이트와 CORS 검증이 때리는 주소 (`https://api.postpilot.<도메인>`) |
| `WEB_ORIGIN` | GitHub repo **variable** | CORS preflight가 흉내낼 브라우저 origin (`https://postpilot.<도메인>`) — 스택 `.env`의 `CORS_ORIGIN`과 같아야 한다 |
| `VITE_API_URL` | Cloudflare Worker → Settings → Build → Variables | 프론트 빌드 타임 주입(번들에 박히는 공개값) |
| `postpilot build token` | Cloudflare가 자동 관리 (Worker → Settings → Build → API token) | Workers Builds 배포 인증. 빌드가 10001 인증 에러로 죽으면 여기서 재발급 |
| 스택 `.env` | VPS `/srv/postpilot-{staging,prod}/.env` (`chmod 600`, 비추적) | 런타임 설정 — 키 목록은 `.env.production.example` |
| edge `.env` | VPS `/srv/edge/.env` | `API_DOMAIN_PROD`/`API_DOMAIN_STAGING` (도메인뿐) |
| GHCR pull PAT (`read:packages`, classic) | VPS `ubuntu` 계정의 docker 로그인 | VPS가 private 이미지를 pull. **sudo 없이** `docker login` |

## 4. VPS 내부 구조

```
/srv/
├── edge/                        # 공유 Caddy — 80/443의 유일한 소유자 (수동 관리)
│   ├── docker-compose.yml       # 리포 deploy/edge/에서 복사
│   ├── Caddyfile                # 두 api 도메인 TLS + h2c 프록시
│   └── .env                     # API_DOMAIN_PROD / API_DOMAIN_STAGING
├── postpilot-staging/           # (지금은 안 씀) staging 스택
│   ├── docker-compose.prod.yml  # 배포 워크플로가 매번 동기화한다 — 손으로 복사하지 않는다
│   ├── .env                     # chmod 600
│   └── data/                    # SQLite 파일. uid 65532 소유여야 한다
└── postpilot-prod/              # main이 배포되는 스택 (구성 동일)
```

- 스택 `.env` 키: `IMAGE_TAG`(배포가 갱신) · `PORT=8080` · `API_UPSTREAM`
  (`postpilot-api-staging`|`postpilot-api-prod` — edge 네트워크에서의 DNS 별칭) ·
  `CORS_ORIGIN`(해당 환경 프론트 origin).
- `data/`는 **uid 65532 소유**여야 한다. 이미지가 distroless:nonroot로 돌기 때문에,
  root 소유로 두면 컨테이너는 뜨는데 첫 쓰기에서 죽는다.
  `sudo install -d -o 65532 -g 65532 /srv/postpilot-<env>/data`
- 도커 외부 네트워크 `edge`(`docker network create edge`)로 Caddy↔api가 통신한다.
  **Caddy는 스택마다 띄우지 않는다** — 80/443 충돌.
- 이미 cosimosi용 edge Caddy가 도는 VPS라면 `deploy/edge/Caddyfile`의 두 블록을
  기존 `/srv/edge/Caddyfile`에 **덧붙이고** `.env`에 도메인 변수를 추가한 뒤
  `docker compose up -d` 만 하면 된다.

## 5. 처음부터 세우기

전제: 리포를 GitHub에 올림, Cloudflare에 도메인, GHCR PAT(`read:packages`, classic).

1. **VPS**: Ubuntu 24.04 LTS + Static IP, 방화벽 22/80/443. (기존 박스 재사용이면 생략)
2. **DNS**(Cloudflare): `api.postpilot`·`api.staging.postpilot` A 레코드 → Static IP,
   둘 다 **DNS only(회색)** — 주황 구름이면 Caddy 인증서 발급이 실패한다.
3. **서버 부트스트랩** (기존 박스 재사용이면 `mkdir`/`chown`만):
   ```bash
   ssh ubuntu@$IP 'set -e
   curl -fsSL https://get.docker.com | sudo sh
   sudo usermod -aG docker ubuntu
   docker network create edge || true
   sudo mkdir -p /srv/edge /srv/postpilot-staging /srv/postpilot-prod
   sudo chown -R ubuntu:ubuntu /srv'
   ```
4. **파일 배치 + .env**: edge만 손으로 복사한다. 스택의 compose 파일은 **배포 워크플로가
   매번 동기화하므로 복사하지 않는다.**
   ```bash
   scp deploy/edge/docker-compose.yml deploy/edge/Caddyfile ubuntu@$IP:/srv/edge/
   ssh ubuntu@$IP 'sudo install -d -o 65532 -g 65532 /srv/postpilot-prod/data'
   ```
   - `/srv/edge/.env`: `API_DOMAIN_PROD=…`, `API_DOMAIN_STAGING=…`
   - 스택별 `/srv/postpilot-<env>/.env`: `.env.production.example`을 채워서 (`chmod 600`).
     `API_UPSTREAM`이 스택마다 **달라야** 한다 — Caddyfile의 업스트림 이름과 일치.
     `CORS_ORIGIN`은 repo variable `WEB_ORIGIN`과 **같아야** 한다 — 다르면 배포 마지막
     단계의 CORS 검증이 실패한다.
5. **GHCR 로그인**(VPS에서, **sudo 없이**): `echo '<PAT>' | docker login ghcr.io -u <계정> --password-stdin`
6. **배포 키**: `ssh-keygen -t ed25519 -f ~/.ssh/postpilot-deploy -N "" -C postpilot-github-actions-deploy`
   → 공개키를 VPS `~/.ssh/authorized_keys`에 추가.
7. **GitHub** (Settings → Secrets and variables → Actions): secrets `SSH_HOST`, `SSH_USER`(`ubuntu`),
   `SSH_KEY`(개인키 내용) → variables `API_ORIGIN`, `WEB_ORIGIN` → 전부 끝난 뒤
   `DEPLOY_ENABLED=true`.
   - prod에 수동 승인을 걸고 싶으면 Settings → Environments → `production`에 리뷰어를
     지정한다. rollout job이 그 environment를 쓴다.
8. **기동**: `/srv/edge`에서 `docker compose up -d`. 스택의 api는 첫 배포가 알아서 띄운다
   (compose 파일 동기화 → pull → up). 이후는 머지가 알아서 배포한다(§2).
9. **Cloudflare Worker**(프론트): 리포 import(이름 `postpilot` = `wrangler.jsonc`의 name),
   production 브랜치 `main`, build `pnpm --filter ./frontend build`,
   deploy `npx wrangler deploy`, version `npx wrangler versions upload`,
   변수 `VITE_API_URL` 입력, 커스텀 도메인 연결.

확인: `curl https://api.postpilot.<도메인>/health` → `{"status":"ok","version":"0.0.1"}`

## 6. 롤백

- **백엔드**: VPS `/srv/postpilot-<env>/.env`의 `IMAGE_TAG=<이전 SHA>`로 바꾸고
  `docker compose -f docker-compose.prod.yml pull && … up -d`.
  (GHCR 이미지는 커밋 SHA로 태깅돼 있다.)
- **프론트**: Worker → Deployments → 이전 버전으로 rollback/promote.

## 7. 아직 없는 것 (의도적)

지금 뼈대에 **일부러 넣지 않은** 것들. 필요해지는 시점에 붙인다.

- **DB** — 스키마와 코드는 아직 없지만 **배포 쪽 자리는 이미 잡혀 있다**: 스택 볼륨의
  `data/`, embed된 마이그레이션이 기동 시 돌고 `/health` 게이트가 실패를 잡는 rollout.
  코드에서 할 일은 `internal/db`(SQLite + goose embed)뿐이다 — 워크플로는 안 건드려도 된다.
- **인증** — 지금은 인터셉터가 없다. 붙일 때 `rpcserver`에 auth 인터셉터를 추가하고,
  프론트 transport에 Bearer 인터셉터를 단다.
- **오브젝트 스토리지** — 사진 원본 저장. 업로드가 생기면 `maxRequestBytes`(현재 256KiB)도
  같이 다시 본다.
- **관측(Sentry/PostHog)** — 사용자가 생기면.
