# 배포 — postpilot 운영 문서

The default remains the existing CPU VPS, with API/SQLite and a separate CPU
media worker. Repository changes provide images, configuration and locally verified
procedures; the operator applies them to each host. Start with [§8](#8-media-worker-deployment)
for the worker upgrade. Secrets stay in restrictive, untracked env files.

구조는 cosimosi와 동일하다. VPS를 이미 쓰고 있다면 **같은 박스·같은 edge Caddy를
그대로 재사용**하면 된다(§5의 3번 부트스트랩을 건너뛰고, edge는 §4의 공유 절차를 따른다 —
`/srv/edge`를 덮어쓰면 cosimosi가 같이 내려간다).

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
build    validate all layouts → build API + CPU worker → separate GHCR SHA tags
rollout  sync Compose/controller files to the API VPS (no SSH to a worker PC)
         → validate env and selected image contracts before downtime
         → pull selected services → drain a local worker → replace API
         → API /health + private compatible-worker status smoke
         → start/check the colocated worker when selected
         → failure: restore the supported image/env/config snapshot and health-check it
verify   browser-origin API CORS and R2 GET preflight checks
```

**일반적인 캐시 재사용 배포는 push부터 위 상태 검사 완료까지 5분 이내를 목표로 한다.**
Actions의 `Deploy backend` 성공이 실제 배포 완료다. 긴 영상 검증은 성공한 배포의 정확한
커밋 SHA로 별도 `Verify media` 워크플로에서 실행한다. 이미지 스모크·워커 검증·두 배치의
릴리스 검증은 서로 병렬이고, 그 결과나 실행 시간이 다음 배포를 붙잡지 않는다.
배포 컨트롤러·워크플로 회귀 테스트는 기존 `CI`에서 계속 실행한다.

실제 서버를 바꾸는 `rollout`만 환경별로 잠그며 진행 중인 교체는 자동 취소하지 않는다.
새 push가 오면 이전 빌드는 취소할 수 있고, 잠금을 얻은 오래된 실행도 더 최신 배포 요청이
있으면 서버 변경을 건너뛴다. 문서만 바뀐 커밋은 새 배포 요청으로 취급하지 않는다.
`Verify media`는 별도 잠금을 사용하고, 새 검증이 오면 이전 검증을 취소할 수 있다.

캐시는 API·워커·검증용 저장소·릴리스 이미지별로 나눠 보관한다. 릴리스 검증의 이미지도
같은 Buildx에서 캐시를 읽어 빌드한 뒤 `--skip-build --skip-storage-build`로 실행하므로
다른 Docker 빌더에서 FFmpeg/MinIO를 다시 컴파일하지 않는다. 검증만 다시 실행하려면
Actions → **Verify media → Run workflow**를 사용한다.

GitHub 실행기 대기, 비어 있는 캐시에서의 최초 도구 컴파일, 느린 레지스트리·네트워크는
5분 목표를 넘길 수 있다. 5분이 지났다는 이유로 진행 중인 서버 교체를 강제 종료하지 않는다.
시간을 확인할 때는 `Deploy backend`의 생성 시각부터 완료 시각까지를 측정하고,
`Verify media`의 소요 시간은 별도로 본다. 검증 실패는 해당 워크플로에 실패로 남는다.

- **마이그레이션 스텝이 따로 없는 건 의도다.** DB가 이 스택 볼륨 안의 SQLite 파일이라
  맞춰야 할 공용 서버가 없다. 마이그레이션은 API 바이너리에 embed되어 기동 시 돈다 —
  스키마가 그걸 읽는 코드와 어긋날 수 없다. 실패하면 프로세스가 죽고, `/health` 게이트가
  잡아내고, rollout이 이전 태그로 되돌린다.
- **CORS 검증이 rollout과 분리된 것도 의도다.** CORS 불일치는 이미지 문제가 아니라 스택
  `.env` 문제라 되돌려도 안 고쳐진다. 시끄럽게 실패해야 한다. 세션 쿠키가 정확한
  origin + credentials 허용에 의존하므로(PRD F-1), 이게 "로그인이 조용히 안 되는" 사고를
  잡는 검사다.
- **CORS 검증이 두 개인 것도 의도다.** API 쪽은 스택 `.env`의 `CORS_ORIGIN`, 버킷 쪽은
  R2 버킷의 CORS 규칙 — 서로 다른 곳에 있고 각각 다른 기능을 조용히 죽인다. 버킷에 `GET`
  허용이 없으면 업로드도 되고 사진도 화면에 보이는데 **사진 복사만** 실패한다(§5). 버킷
  검사는 presigned URL이 실제로 서명되는 호스트(`R2_PUBLIC_ENDPOINT`가 있으면 그것,
  없으면 `R2_ENDPOINT`)를 때린다 — 브라우저가 가는 곳이 거기라서다.
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
| `CLIENT_IP_HEADER` | 스택 `.env` | 인증 요청 IP 기준. 로컬은 비워 direct peer를 쓰고, 유일한 ingress가 Caddy인 배포는 `X-Forwarded-For` |
| `VITE_API_URL` | Cloudflare Worker → Settings → Build → Variables | 프론트 빌드 타임 주입(번들에 박히는 공개값) |
| `GOOGLE_CLIENT_ID` | 스택 `.env` | Google OAuth 웹 클라이언트 ID. secret과 둘 다 비우면 Google 로그인이 꺼진다 |
| `GOOGLE_CLIENT_SECRET` | 스택 `.env` | Google OAuth 웹 클라이언트 secret. ID와 한쪽만 설정하면 API가 기동하지 않는다 |
| `VITE_GOOGLE_CLIENT_ID` | Cloudflare Worker → Settings → Build → Variables | 같은 Google OAuth 클라이언트의 공개 ID. 비우면 로그인·회원가입의 Google 버튼이 숨겨진다 |
| `TOSS_SECRET_KEY` / `TOSS_CLIENT_KEY` | 스택 `.env` | Toss Payments 자동결제 API 키. 둘 다 `EXIM_API_KEY`와 함께 설정하거나 모두 비운다 |
| `VITE_TOSS_CLIENT_KEY` | Cloudflare Worker → Settings → Build → Variables | 카드 등록창에 쓰는 공개 Toss client key. 서버의 `TOSS_CLIENT_KEY`와 같은 값 |
| `EXIM_API_KEY` | 스택 `.env` | 한국수출입은행 전일 매매기준율 조회 키. Toss 키와 함께 설정하지 않으면 결제 quote/write가 비활성화된다 |
| `NAVER_SEARCH_CLIENT_ID` / `NAVER_SEARCH_CLIENT_SECRET` | 스택 `.env` | 네이버 개발자센터에 등록한 애플리케이션의 검색 API 키(사용 API에 **검색**이 켜져 있어야 한다 — 403이면 꺼진 것). 매일 도는 분야 문구 배치만 쓴다. 둘 중 하나라도 비면 배치가 돌지 않고 모든 문구 목록이 비어 있을 뿐, API는 그대로 뜬다 |
| `QUALITY_PHRASE_REFRESH_INTERVAL` | 스택 `.env` (선택) | 분야마다 문구 목록을 새로 받는 주기. 비우면 제품 기본값 24h. 형식이 틀렸거나 0·음수이거나 1h 미만이면 API가 기동하지 않는다 |
| `postpilot build token` | Cloudflare가 자동 관리 (Worker → Settings → Build → API token) | Workers Builds 배포 인증. 빌드가 10001 인증 에러로 죽으면 여기서 재발급 |
| 스택 `.env` | VPS `/srv/postpilot-{staging,prod}/.env` (`chmod 600`, 비추적) | 런타임 설정 — 키 목록은 `.env.production.example` |
| edge `.env` | VPS `/srv/edge/.env` (박스 공유 — **덮어쓰지 말고 append**) | `POSTPILOT_API_DOMAIN_PROD`(+staging을 띄울 때만 `..._STAGING`). 도메인만, 접두사 필수 — 이유는 §4. 템플릿: `deploy/edge/.env.example` |
| GHCR pull PAT (`read:packages`, classic) | VPS `ubuntu` 계정의 docker 로그인 | VPS가 private 이미지를 pull. **sudo 없이** `docker login` |
| `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` | 스택 `.env` | R2 API 토큰(해당 버킷에만 Object Read & Write). Cloudflare → R2 → Manage API Tokens |
| `R2_ENDPOINT` / `R2_BUCKET` | 스택 `.env` | `https://<account-id>.r2.cloudflarestorage.com` 과 버킷 이름. 비밀은 아니지만 환경마다 다르다 |
| `PUBLISH_*` | 스택 `.env` | 폐기 브리지 동안 구 설정을 읽기 위한 값. migration 0072 이후 실행에는 사용되지 않으며 최종 제거 단계에서 삭제한다 |
| `MAIL_DRIVER` | 스택 `.env` | 트랜잭션 메일 전송기. 로컬은 `log`, 배포는 `resend` |
| `RESEND_API_KEY` / `MAIL_FROM` | 스택 `.env` | Resend API 키와 인증된 발신 주소. `MAIL_DRIVER=resend`이면 둘 다 필수 |
| `OPENROUTER_API_KEY` (외 `backend/config/providers.yaml`의 `api_key_env`가 가리키는 이름들) | 스택 `.env` | 모델 프로바이더 키. **없어도 API는 뜬다** — 그 프로바이더의 모델만 드롭다운에서 "API key not configured"로 비활성. 이미지는 `/config/providers.yaml`을 내장하며(`PROVIDERS_CONFIG`), 스택이 자기 파일을 그 자리에 마운트해 덮어쓸 수 있다 |

## 4. VPS 내부 구조

```
/srv/
├── edge/                        # 공유 Caddy — 80/443의 유일한 소유자. **박스 소유**, 수동 관리
│   ├── docker-compose.yml       # 박스 것 — 프로젝트가 늘어도 안 고친다
│   ├── Caddyfile                # 라우터뿐: `import conf.d/*.caddy`
│   ├── conf.d/                  # 프로젝트당 파일 하나 (각 리포가 자기 것만 소유)
│   │   ├── cosimosi.caddy       #   ← cosimosi 리포
│   │   └── postpilot.caddy      #   ← 이 리포 (deploy/edge/conf.d/에서 복사)
│   └── .env                     # 모든 프로젝트의 도메인 변수 (접두사 필수, 도메인만)
├── postpilot-staging/           # (지금은 안 씀) staging 스택
│   ├── docker-compose.prod.yml  # 배포 워크플로가 매번 동기화한다 — 손으로 복사하지 않는다
│   ├── docker-compose.media.colocated.yml / docker-compose.media.remote.yml
│   ├── worker.env               # chmod 600, execution-only identity/settings
│   ├── .deploy/                 # chmod 700, current/previous deployment snapshots
│   ├── .env                     # chmod 600
│   └── data/                    # SQLite 파일. uid 65532 소유여야 한다
└── postpilot-prod/              # main이 배포되는 스택 (구성 동일)
```

- 스택 `.env` 키: `IMAGE_TAG`(배포가 갱신) · `PORT=8080` · `API_UPSTREAM`
  (`postpilot-api-staging`|`postpilot-api-prod` — edge 네트워크에서의 DNS 별칭) ·
  `CORS_ORIGIN`(해당 환경 프론트 origin).
- 스택 `.env`에 `DB_PATH=/data/postpilot.db`도 넣는다 (`.env.production.example` 참고).
- `data/`에는 SQLite 파일(`postpilot.db` + WAL 사이드카)이 들어간다. 백업 대상은 이 디렉터리 하나다.
- `data/`는 **uid 65532 소유**여야 한다. 이미지가 distroless:nonroot로 돌기 때문에,
  root 소유로 두면 컨테이너는 뜨는데 첫 쓰기에서 죽는다.
  `sudo install -d -o 65532 -g 65532 /srv/postpilot-<env>/data`
- **계정 생성**: 가입 화면이 없으므로(PRD F-1) 운영자가 직접 만든다. api 이미지의
  `ENTRYPOINT ["/api"]`가 `adduser` 서브커맨드를 받는다 — 별도 이미지가 필요 없다.
  ```bash
  cd /srv/postpilot-<env>
  docker compose -f docker-compose.prod.yml run --rm api adduser <login_id>
  # 비밀번호를 두 번 입력한다 (TTY면 에코 없음). 중복 id는 non-zero로 거절된다.
  ```
- 도커 외부 네트워크 `edge`(`docker network create edge`)로 Caddy↔api가 통신한다.
  **Caddy는 스택마다 띄우지 않는다** — 80/443 충돌.
- **edge는 프로젝트가 몇 개로 늘어도 확장된다.** `/srv/edge/Caddyfile`은 site 블록을
  담지 않고 `import conf.d/*.caddy` 한 줄만 있다. 프로젝트를 하나 얹는 일은
  **파일 하나 놓기 + `.env` 한 줄**이고, 남의 파일은 건드리지 않는다.

  | 파일 | 주인 |
  |---|---|
  | `Caddyfile`, `docker-compose.yml` | **박스**. 프로젝트별로 고치지 않는다 (전역 옵션 추가만 예외) |
  | `conf.d/<프로젝트>*.caddy` | 그 프로젝트 리포. 자유롭게 만들고 지운다 |
  | `.env` | 공유. **자기 접두사 변수만 append** |

- **postpilot을 이미 도는 박스에 얹기** (edge가 이미 conf.d 구조일 때):
  ```bash
  scp deploy/edge/conf.d/postpilot.caddy ubuntu@$IP:/srv/edge/conf.d/
  ssh ubuntu@$IP 'echo POSTPILOT_API_DOMAIN_PROD=api.postpilot.<도메인> >> /srv/edge/.env'
  ```
  반영 **전에** 조립된 설정을 검증한다. Caddy는 all-or-nothing이라 프래그먼트 하나가
  깨지면 **박스의 모든 프로젝트가 TLS를 잃는다** — 이 검증이 그걸 막는 유일한 장치다.
  ```bash
  docker run --rm -v /srv/edge/Caddyfile:/etc/caddy/Caddyfile:ro \
    -v /srv/edge/conf.d:/etc/caddy/conf.d:ro --env-file /srv/edge/.env \
    caddy:2-alpine caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
  ```
  `Valid configuration`을 본 뒤 반영한다. **`.env`가 바뀌었으면** 컨테이너를 새로 만들어야
  변수가 들어간다(`caddy reload`는 기존 프로세스의 env를 그대로 본다):
  `cd /srv/edge && docker compose up -d`. **`conf.d/`만 바뀐 경우**는 무중단 리로드로 끝난다:
  `docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile`.

- **변수 규칙 두 개.** 둘 다 어기면 박스 전체가 내려간다 — 자기 프로젝트만 안 뜨는 게 아니다.
  1. **접두사 필수** (`POSTPILOT_API_DOMAIN_PROD`). `.env`가 공유라 bare `API_DOMAIN_PROD`는
     형제 프로젝트와 충돌해 두 site 블록이 같은 주소로 전개되고,
     Caddy가 `ambiguous site definition`으로 **설정 전체를 거부**한다.
  2. **설치한 프래그먼트의 변수는 반드시 설정.** 미설정 변수는 빈 site 주소로 전개돼
     `server block without any key`로 역시 전체가 거부된다. 그래서 staging 프래그먼트는
     `postpilot.staging.caddy.disabled`로 들어 있다 — `import`가 `*.caddy`만 잡으므로
     **conf.d를 통째로 복사해도 안전**하고, 켜는 건 `.disabled`를 떼는 rename이다
     (순서: `.env`에 변수 먼저 → rename → validate → `up -d`).
     conf.d가 아예 비어 있는 것도 안전하다: glob 미스는 경고만 남기고 통과한다.

- **edge가 아직 conf.d 구조가 아니라면**(`/srv/edge/Caddyfile`에 site 블록이 직접 들어 있는
  옛 모양) 그 전환은 **박스 주인 프로젝트가** 한다 — postpilot이 남의 파일을 재배치하지
  않는다. 전환은 이렇다: 기존 site 블록을 `conf.d/<그 프로젝트>.caddy`로 옮기고, Caddyfile은
  `import conf.d/*.caddy`만 남기고, compose에 `env_file: .env`와
  `./conf.d:/etc/caddy/conf.d:ro`를 추가하고, 그 프로젝트의 도메인 변수에도 접두사를 붙인다
  (`.env`와 프래그먼트를 **같이** 바꿔야 한다). 위 validate를 통과한 뒤 `up -d`.

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
   sudo mkdir -p /srv/edge/conf.d /srv/postpilot-staging /srv/postpilot-prod
   sudo chown -R ubuntu:ubuntu /srv'
   ```
4. **파일 배치 + .env**: edge만 손으로 복사한다. 스택의 compose 파일은 **배포 워크플로가
   매번 동기화하므로 복사하지 않는다.**
   ```bash
   scp deploy/edge/docker-compose.yml deploy/edge/Caddyfile ubuntu@$IP:/srv/edge/
   scp deploy/edge/conf.d/postpilot.caddy ubuntu@$IP:/srv/edge/conf.d/
   ssh ubuntu@$IP 'sudo install -d -o 65532 -g 65532 /srv/postpilot-prod/data'
   ```
   - `/srv/edge/.env`: `deploy/edge/.env.example`을 채운다 — 지금은
     `POSTPILOT_API_DOMAIN_PROD` 한 줄. staging은 `.disabled`를 떼는 순간 활성화된다
     (그때 `..._STAGING`을 추가한다).
   - **기존 박스 재사용이면 이 단계 대신 §4의 "이미 도는 박스에 얹기"를 따른다** —
     `Caddyfile`/`docker-compose.yml`은 그 박스 것이라 덮어쓰지 않는다.
   - 스택별 `/srv/postpilot-<env>/.env`: `.env.production.example`을 채워서 (`chmod 600`).
     `DB_PATH`는 예제 값(`/data/postpilot.db`) 그대로 두면 된다 — 위 볼륨 마운트와 짝이다.
     `API_UPSTREAM`이 스택마다 **달라야** 한다 — `deploy/edge/conf.d/postpilot*.caddy`의
     `reverse_proxy` 업스트림 이름과 일치.
     `CORS_ORIGIN`은 repo variable `WEB_ORIGIN`과 **같아야** 한다 — 다르면 배포 마지막
     단계의 CORS 검증이 실패한다.
5. **R2 버킷**(Cloudflare → R2): 환경별 버킷을 만들고(`postpilot-prod`), 그 버킷에만
   Object Read & Write 권한을 가진 API 토큰을 발급해 스택 `.env`에 넣는다.
   **버킷은 공개하지 않는다** — 사진은 API가 소유자를 확인한 뒤 발급하는 presigned URL로만
   읽힌다(PRD F-5).

   **CORS 규칙을 반드시 넣는다.** 규칙은 이 리포의 [`deploy/r2-cors.json`](deploy/r2-cors.json)이
   소유한다 — 대시보드에 손으로 붙여넣지 말고 그 파일을 적용한다:
   ```bash
   # origins의 https://postpilot.<domain> 자리를 실제 web origin(= repo variable WEB_ORIGIN)으로
   # 바꾼 뒤 (환경이 여러 개면 배열에 전부 나열한다), 버킷마다 적용한다.
   npx wrangler r2 bucket cors set postpilot-prod --file deploy/r2-cors.json
   npx wrangler r2 bucket cors list postpilot-prod   # 적용된 규칙 되읽기
   ```
   `methods`는 브라우저가 실제로 쓰는 세 가지다 — `PUT`(업로드), `GET`(사진 읽기·복사),
   `HEAD`(preflight 뒤 확인). **셋 중 하나만 빠져도 조용히 깨진다**: 규칙이 아예 없으면
   서버 쪽 단계는 전부 성공했다고 보고하는데 업로드만 실패하고(PRD F-2), `GET`만 빠지면
   업로드는 되는데 **내보내기 패널의 사진이 아예 뜨지 않는다**(plan 08). 사진 복사는 화면에
   이미 그려진 픽셀을 읽으므로 그 `<img>`가 `crossOrigin`으로 로드돼야 하고, 그래서 이
   패널에서는 CORS 허용이 복사뿐 아니라 **표시**의 조건이다 — 다른 화면의 썸네일은 평범한
   `<img>`라 규칙 없이도 계속 보이므로, "목록에는 보이는데 내보내기에서만 안 보인다"가
   이 규칙이 빠졌을 때의 증상이다. 배포 마지막 단계가 `GET` preflight를 검증하므로 `GET`이
   빠진 버킷은 배포가 실패한다(§2).
   클립 원본 업로드는 `Content-Type`과 `If-None-Match: *`를 서명해 확인된 원본의
   덮어쓰기를 막는다. `allowed.headers`의 `if-none-match`도 함께 적용해야 한다.
   저장소 정책은 자동 변경하지 않으므로 클립 기능을 배포하기 전에 갱신된 파일을 적용한다.
   > `wrangler r2 bucket cors set --file`이 받는 JSON은 대시보드가 쓰는
   > `[{"AllowedOrigins": …}]` 배열이 아니라 `{"rules":[{"allowed":{…}}]}` 형태다.
   > `deploy/r2-cors.json`이 그 형태로 들어 있다.

   (로컬 개발은 R2가 아니라 compose의 MinIO를 쓴다 — `docker-compose.yml`. MinIO는 기본으로
   모든 origin을 허용하므로 개발 origin `http://localhost:2564`에 대한 별도 CORS 규칙은
   필요 없다.)
6. **GHCR 로그인**(VPS에서, **sudo 없이**): `echo '<PAT>' | docker login ghcr.io -u <계정> --password-stdin`
7. **배포 키**: `ssh-keygen -t ed25519 -f ~/.ssh/postpilot-deploy -N "" -C postpilot-github-actions-deploy`
   → 공개키를 VPS `~/.ssh/authorized_keys`에 추가.
8. **GitHub** (Settings → Secrets and variables → Actions): secrets `SSH_HOST`, `SSH_USER`(`ubuntu`),
   `SSH_KEY`(개인키 내용) → variables `API_ORIGIN`, `WEB_ORIGIN` → 전부 끝난 뒤
   `DEPLOY_ENABLED=true`.
   - prod에 수동 승인을 걸고 싶으면 Settings → Environments → `production`에 리뷰어를
     지정한다. rollout job이 그 environment를 쓴다.
9. **기동**: `/srv/edge`에서 `docker compose up -d`. 스택의 api는 첫 배포가 알아서 띄운다
   (compose 파일 동기화 → pull → up). 이후는 머지가 알아서 배포한다(§2).
10. **Cloudflare Worker**(프론트): 리포 import(이름 `postpilot` = `wrangler.jsonc`의 name),
   production 브랜치 `main`, build `pnpm --filter ./frontend build`,
   deploy `npx wrangler deploy`, version `npx wrangler versions upload`,
   변수 `VITE_API_URL`, Google 로그인을 쓸 때 `VITE_GOOGLE_CLIENT_ID` 입력, 커스텀 도메인 연결.
   Google Cloud Console의 같은 OAuth 웹 클라이언트에 `<web origin>/login/google/callback`
   (예: `https://postpilot.haeram.me/login/google/callback`)을 승인된 리디렉션 URI로 등록한다.

11. **첫 계정**: 배포가 끝나면 §4의 `adduser`로 계정을 만든다. 만들기 전까지는 로그인할
    방법이 없다 — 마이그레이션은 기동 때 이미 돌았으므로 이 단계만 남는다.

확인: `curl https://api.postpilot.<도메인>/health` → `{"status":"ok","version":"0.0.1"}`
(인증이 필요한 RPC는 세션 쿠키 없이 부르면 401이다 — `/health`만 열려 있다.)

### 자동 발행 폐기 브리지

migration 0072부터 자동 발행은 영구 차단된다. 아래 보고·정리 명령은 `/retirepublishing`을 포함한
**T312 bridge 이미지**에서만 실행한다. 최종 이미지에는 이 명령이 없으므로 bridge image SHA를 환경별
checkpoint가 끝날 때까지 보관한다. 배포 전 구 API 프로세스를 완전히 내리고 bridge 이미지
하나만 기동한다.

**bridge 이미지가 GHCR에 없을 때.** T310→T283 체인이 한 번의 푸시로 main에 올라가면 그 사이 커밋에는
deploy 실행이 없고, 따라서 bridge 이미지가 GHCR에 존재하지 않는다(prod에서 실제로 이렇게 됐다 — 아래
checkpoint 참고). 이때는 bridge 커밋에서 정적 바이너리만 만들어 현재 이미지에 bind-mount하면 같은
경계를 만족한다. 컨테이너는 그대로 distroless/nonroot로 돌고, 명령·검증·영수증은 전부 동일하다.

```bash
git worktree add --detach /tmp/postpilot-bridge <T312-bridge-commit-SHA>
cd /tmp/postpilot-bridge/backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o /tmp/retirepublishing ./cmd/retirepublishing
scp /tmp/retirepublishing <vps>:/home/ubuntu/retirepublishing-bridge-<sha>
```

그러면 아래 모든 명령에서 `IMAGE_TAG=<T312-bridge-image-SHA>` 대신
`-v /home/ubuntu/retirepublishing-bridge-<sha>:/retirepublishing:ro`를 붙이고 `--no-deps`로 실행한다.
`/data`의 보고서·inventory·receipt는 mode 0600에 uid 65532 소유여야 하므로, 호스트에서 만들지 말고
`docker run --rm -u 65532:65532 -v /srv/postpilot-<env>/data:/data ... sh -c 'umask 077; ...'`로 쓴다. 이 경계에서는 구 이미지와 새 이미지를 동시에 실행하지 않는다. 마이그레이션은
연결 코드와 에이전트 토큰을 무효화하고, 커밋 전 작업은 `canceled`, 커밋 가능성이 있는 작업은
`outcome_unknown`으로 고정한다. 구 클라이언트의 SQL 쓰기도 트리거가 거절한다.

새 이미지의 `/health`를 확인한 직후 환경별 비공개 보고서를 한 번 만든다. 출력 파일은 기존 파일을
덮어쓰지 않으며 mode 0600이다. 내용에는 안전한 연결·작업 식별자와 상태만 있고 글 내용, manifest,
미디어, 토큰, 브라우저 경로는 없다.

```bash
cd /srv/postpilot-<env>
IMAGE_TAG=<T312-bridge-image-SHA> docker compose -f docker-compose.prod.yml run --rm \
  --entrypoint /retirepublishing api report \
  --environment <env> \
  --output /data/publishing-retirement-<env>.json
```

이미지 SHA, migration 0072 적용 여부, 명령이 출력한 digest와 보고서 파일의 보관 위치를 환경별로
기록한다. 저장된 Naver URL과 `outcome_unknown` 결과는 운영자가 Naver에서 직접 확인하며 자동 재시도나
삭제를 하지 않는다. 설치된 Mac 동반 프로그램을 모두 중지했다는 영수증이 모이기 전에는 staged
object나 발행 레코드를 지우지 않는다. prod는 아래 checkpoint로 실행을 마쳤고, staging은 아직 배포된
적이 없어 해당 없음이다.

보고서에 대응하는 모든 Mac에서는 **T282 retirement bridge가 들어간 동일한 검토 커밋 SHA**를 따로
기록하고, 현재 main이 아니라 그 보관 커밋을 임시 worktree로 체크아웃해 읽기 전용 점검을 실행한다.
최종 소스에는 `agent/`가 없다. 예전 `install.sh`로 바이너리를 교체하거나 LaunchAgent를 다시 올리지 않는다.

```bash
git worktree add --detach /tmp/postpilot-agent-retirement <T282-bridge-commit-SHA>
cd /tmp/postpilot-agent-retirement/agent
go run ./cmd/postpilot-agent retire
# 표시된 credential 수와 보존될 browser profile 경로를 Mac 소유자가 확인한 뒤
go run ./cmd/postpilot-agent retire --apply
```

`--apply`는 현재 사용자 LaunchAgent와 확인된 수동 companion을 먼저 중지·재확인하고, mode 0600 로컬
영수증을 `~/Library/Application Support/Postpilot Agent Retirement/shutdown-receipt.json`에 남긴다.
브라우저 프로필은 기본적으로 보존한다. 프로필까지 지워야 하는 Mac에서만 점검에 나온 정확한 경로를
확인한 뒤 `retire --apply --delete-profiles`를 별도로 실행한다. 영수증이 `complete`가 아니면 해당 장치는
미해결 상태이며 서버 cleanup을 진행하지 않는다. 재실행은 같은 영수증의 계정·경로 inventory로
idempotent하게 이어진다.

운영 기록에는 Mac별 bridge 커밋 SHA, 영수증 상태, 운영자가 계산한 digest와 장치 식별용 별칭만 적는다.
영수증 파일 자체나 로컬 브라우저 경로는 서버·Git·공유 로그에 업로드하지 않는다. 폐기됐거나 설치된
적이 없는 장치는 별도 inventory reconciliation으로 근거를 남기며, 연락되지 않는 장치를 자동으로
중지 완료로 간주하지 않는다. 이 명령은 Naver나 Postpilot API에 접속하지 않고 실제 발행도 검증하지
않는다.

모든 장치를 해소한 뒤 환경별 shutdown inventory를 mode 0600으로 만든다. `devices`는 최초 보고서의
agent id를 정확히 한 번씩 포함해야 하고, `disposition`은 `shutdown`·`never_installed`·`destroyed` 중
하나다. `evidence_digest`에는 Mac 로컬 영수증 또는 별도 inventory reconciliation의 `sha256:` digest만
기록한다. 로컬 경로와 영수증 원문은 넣지 않는다. agent가 0개인 환경도 빈 `devices` 배열을 가진 파일이
필요하다.

```json
{
  "schema_version": 1,
  "environment": "<env>",
  "report_digest": "<report 명령이 출력한 digest>",
  "devices": [
    {"agent_id": "<agent id>", "disposition": "shutdown", "evidence_digest": "sha256:<64 hex>"}
  ]
}
```

원본 retirement report의 mode 0600 파일과 출력 digest를 그대로 보관한다. cleanup은 그 digest뿐 아니라
현재 database identity, migration 0072 cutoff, agent/job inventory와 다섯 테이블의 row 수를 원본 보고서와
대조한다. shutdown inventory의 정확한 파일 digest도 함께 넘긴다(`sha256sum` 결과 앞에 `sha256:`를 붙임).
기본 실행은 DB와 `publishing/` 전체 목록을 읽기만 하며 파일이나 row를 지우지 않는다.

```bash
cd /srv/postpilot-<env>
IMAGE_TAG=<T312-bridge-image-SHA> docker compose -f docker-compose.prod.yml run --rm \
  --entrypoint /retirepublishing api cleanup \
  --environment <env> \
  --report /data/publishing-retirement-<env>.json \
  --report-digest <report-digest> \
  --shutdown-inventory /data/publishing-shutdown-<env>.json \
  --shutdown-digest sha256:<shutdown-file-hex> \
  --receipt /data/publishing-cleanup-<env>.json
```

점검 결과가 원본 inventory와 일치할 때 같은 명령에 `--apply`를 추가한다. apply는 완전히 pagination된
`publishing/` 목록에서 참조 copy와 orphan을 exact key로 먼저 지우고, 새 전체 목록이 0임을 확인한 뒤
`publish_assets` → `publish_jobs` → `publish_job_ids` → `publishing_agents` → `publishing_pairings` 순서의
짧은 DB transaction을 실행한다. `posts/`, `clip-inputs/`와 다른 prefix는 대상이 아니다. object I/O 중에는
DB transaction을 열지 않는다. 실패 시 mode 0600 cleanup receipt가 이미 처리한 exact key와 증거 digest를
보관하므로 같은 `--apply` 명령을 반복한다. 원본 retirement report는 덮어쓰지 않는다.

apply 성공 후 같은 명령의 `--verify`로 현재 다섯 테이블과 `publishing/` 목록이 모두 0이고 complete receipt의
digest가 같은 증거를 가리키는지 다시 확인한다. 환경별로 bridge image SHA, 원본 report digest, shutdown
inventory digest, complete cleanup receipt digest를 기록한다. 실제 환경에서 이 checkpoint가 끝나기 전에는
publishing 테이블·bridge command·guard를 제거하는 T283 이미지를 배포하지 않는다.

#### prod checkpoint (260922, 완료)

| 항목 | 값 |
| --- | --- |
| bridge 커밋 | `cb0575abbc33e47bcec9bbf04b538b17dfa71644` (GHCR 이미지 없음 — 위 bind-mount 방식) |
| T282 Mac 영수증 | `sha256:bcf0004f31fc76e83d08463f0cb97d2a20e1d2d634a305d0253fdc0ca2ed0768` (`retire --apply --delete-profiles`, status complete) |
| report digest | `c32ebce5609e73f0d53c4324c0c67a8c50ac56f43aba88879b8b230f499d888b` |
| shutdown inventory digest | `sha256:a8579b9c37afbfdec8146dedce4b8252f1b4d463921c8200f76d69539098a4cf` |
| cleanup receipt digest | `93cb45caffcc95548571a3bde8167585b823fe43f21c8823b17e0038f3b9d882` (status complete) |
| 정리 규모 | rows 18 → 0 (pairings 10, agents 4, jobs 2, job_id 2, assets 0), `publishing/` objects 0 |
| 최종 이미지 | `4122c499539255b9b9b1f1ff2ec34b60c73b9bb3`, goose 76 |

보고서·inventory·receipt 원본은 `/srv/postpilot-prod/data/publishing-{retirement,shutdown,cleanup}-prod.json`에
mode 0600으로 남아 있다. 이 checkpoint 이전에 T283 이미지가 먼저 배포돼 `Deploy backend` rollout이
0076에서 실패했고(run 35704843925), 헬스 게이트가 구 이미지로 롤백했다. 순서를 지켰다면 발생하지 않는다.

prod가 유일한 배포 환경이고 그 receipt가 complete이므로 bridge 바이너리는 checkpoint 직후 VPS에서
폐기했다. 다시 필요하면 위 커밋에서 그대로 빌드한다 — 최종 이미지에는 이 명령이 없다.

최종 이미지는 migration 0076에서 다섯 테이블의 row 수가 모두 0인지 다시 확인하고, 그 뒤에만 trigger,
index와 테이블을 FK 순서로 제거한다. migration은 object storage를 읽거나 지우지 않으므로 complete receipt가
`publishing/` 정리의 유일한 배포 증거다. row가 하나라도 남으면 `publishing cleanup required before final
removal`로 기동이 실패하고 migration transaction은 어떤 테이블도 지우지 않는다. 이 경우 final migration을
약화하거나 row를 수동 삭제하지 말고, 해당 환경을 기록된 bridge image SHA로 되돌려 위 inspect → apply →
verify를 반복한다(그 이미지가 GHCR에 없으면 bridge 커밋에서 바이너리를 빌드해 bind-mount한다). 모든
환경의 complete receipt와 final image SHA를 기록한 뒤에만 bridge 이미지를 폐기한다.

## 6. 롤백

- **Backend**: from `/srv/postpilot-<env>`, run
  `API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh --rollback`.
  This restores both service pins, both env files and the matching Compose files from
  `.deploy/previous`, checks compatibility, drains the worker, then verifies recovery.
  Exported incoming tags cannot override that snapshot. Do not restore a tag with a
  direct API-only `compose up`: pre-worker boot is unsafe for parked media jobs. The
  minimum supported media rollback is version 1 (`org.postpilot.media.rollback-safe=1`).
  migration 0072를 지난 환경은 이 경계 아래로 되돌리지 않는다. 구 이미지가 필요해도 데이터베이스의
  폐기 트리거와 무효화된 자격증명을 유지해야 하며 자동 발행을 다시 활성화하는 설정은 없다.
  migration 0076을 지난 환경은 publishing 테이블을 요구하는 bridge/구 바이너리로 롤백할 수 없다. 0076의
  Down도 실행 기능이나 테이블을 복원하지 않으므로, 롤백 대상은 최종 스키마와 호환되는 이미지여야 한다.
- **기동 실패 진단**: `docker compose -f docker-compose.prod.yml logs --no-color --tail 80 api`.
  `migration failed`가 있으면 DB의 `goose_db_version`과 해당 마이그레이션을 확인한다.
  확정된 클립은 변경할 수 없으므로 데이터 정리 마이그레이션에서도 보존해야 한다.
- **프론트**: Worker → Deployments → 이전 버전으로 rollback/promote.

## 7. 아직 없는 것 (의도적)

지금 뼈대에 **일부러 넣지 않은** 것들. 필요해지는 시점에 붙인다.

- **비밀번호 변경/재설정** — PRD에 흐름이 정의돼 있지 않다. 지금은 운영자가 계정을
  다시 만드는 것이 유일한 경로다.
- **R2 백업** — 버킷 자체의 백업은 아직 없다(PRD §9.4). `data/`의 SQLite는 디렉터리
  하나만 받으면 되지만, 사진은 R2에 있다.
- **관측(Sentry/PostHog)** — 사용자가 생기면.


## 8. 영상 워커 배포

<a id="media-environments"></a>
### 8.1 환경 선택과 공통 설정

**현재 기본값은 GPU 없는 기존 VPS 한 대에서 API + CPU 워커를 실행하는 구성이다.** 이번 변경으로 운영 서버를 옮기거나 재배포하지 않는다. 호스트 배치는 `MEDIA_TOPOLOGY`, 이미지 종류는 `MEDIA_WORKER_VARIANT`, 실행 프로필은 `MEDIA_ACCEL`로 선택한다. 환경을 추측해서 드라이버를 설치하거나 다른 서버를 발견하지 않는다.

| 환경 | 호스트별 서비스 | 순서대로 선택하는 Compose 파일 | 절차 |
| --- | --- | --- | --- |
| 1 server without GPU — 기본 | VPS: API, SQLite, Caddy, CPU 워커 | `docker-compose.prod.yml` + `docker-compose.media.colocated.yml` | [8.2](#media-cpu) |
| 1 server with NVIDIA GPU | 같은 호스트: API, SQLite, Caddy, 워커 | 위 두 파일 + `docker-compose.media.nvidia.yml` | [8.3](#media-single-gpu) |
| 1 server + 1 GPU server | VPS: API/SQLite/Caddy, GPU PC: 워커만 | VPS: prod + remote; PC: `docker-compose.yml` + `docker-compose.nvidia.yml` | [8.4](#media-remote-gpu) |

**GPU 후보 이미지에서도 실제 사용자 작업은 현재 CPU로 실행한다.** `cpu`와 `auto`는 CPU 프로필을 사용하고, `nvenc`는 승인되지 않은 프로필이므로 시작을 거부한다. [8.7의 격리 진단](#media-gpu-diagnostics)은 실행할 수 있지만, GPU 자동 선택·장애 후 CPU 전환은 CLIP-163 품질/시간 검증과 다음 구현 이후에 활성화한다.

지원 호스트는 **Linux + Docker Engine + Compose 2.24 이상**, Python 3.9 이상(최초 자동 전환에는 표준 `sqlite3` 모듈 포함), `curl`, GHCR pull 권한이다. Windows/WSL2와 친구 PC의 실제 OS/GPU는 검증하지 않았다. API/워커 이미지 태그는 **게시된 40자리 commit SHA**이고 서로 다른 SHA여도 프로토콜·렌더러·에셋 계약이 같아야 한다. `<api-sha>`, `<worker-sha>`, `<candidate-sha>`, `<token>`, `<api-domain>`, `<vps>.<tailnet>.ts.net`은 모두 실제 값으로 바꿀 자리다. GPU 후보는 별도 수동 워크플로 `media-nvidia-candidate.yml`에서 게시하며, CPU 배포는 그 완료를 기다리지 않는다.

처음 설치하는 API 호스트는 §4–5의 Caddy/edge/DB 소유권/스토리지/메일 설정을 먼저 완료한다. API 전체 env 예제는 [`.env.production.example`](.env.production.example)이며, 기존 VPS에서는 기존 값을 보존하고 아래 배치 설정만 추가한다. API 이미지에는 미리보기·편집안 검증용 미디어 도구도 계속 포함된다.

```dotenv
# VPS의 .env: 전체 파일은 .env.production.example을 사용한다.
# 기존 DB_PATH, CORS_ORIGIN, API_UPSTREAM, R2_*, 메일·인증·모델 값은 보존한다.
IMAGE_TAG=<api-sha>
MEDIA_TOPOLOGY=colocated
MEDIA_WORKER_CREDENTIALS={"prod-cpu-1":"<token>"}
# 비우면 R2_ENDPOINT 사용. 원격 PC에서 접근 가능한 서명용 주소여야 한다.
MEDIA_STORAGE_ENDPOINT=
```

아래는 워커의 **전체 env 예제**다. 기존 CPU VPS의 첫 자동 배포에서는 이 파일과 토큰을 자동으로 준비한다. 직접 설정하거나 GPU/원격 구성을 준비할 때는 `deploy/media/worker.env.example`에서 복사해 스택 디렉터리의 `worker.env`로 둔다. GPU/원격 절차에서는 명시한 값만 바꾼다. API의 `.env`, DB, R2 키, 모델·결제 키를 PC로 복사하지 않는다. VPS에도 `worker.env`를 남겨 워커 이미지 호환성을 검사한다.

```dotenv
MEDIA_WORKER_IMAGE_TAG=<worker-sha>
MEDIA_WORKER_VARIANT=cpu
MEDIA_API_URL=http://api:9000
MEDIA_WORKER_ID=prod-cpu-1
MEDIA_WORKER_TOKEN=<token>
MEDIA_ACCEL=cpu
MEDIA_WORKER_CONCURRENCY=1
MEDIA_WORKER_CPUS=1.0
MEDIA_WORKER_MEMORY=512m
MEDIA_DRAIN_TIMEOUT=30s
MEDIA_STOP_TIMEOUT=45
CLIP_WORK_ROOT=/var/lib/postpilot-media/work
CLIP_WORK_STALE_AGE=6h
CLIP_MEDIA_TIMEOUT=15m
CLIP_ENCODE_THREADS=1
CLIP_DECODE_THREADS=2
```

토큰은 `python3 -c 'import secrets; print(secrets.token_urlsafe(32))'`로 생성한다. 환경/워커별 ID와 토큰을 따로 쓰고 API `MEDIA_WORKER_CREDENTIALS`의 해당 항목과 일치시킨다. 두 실행기에 같은 ID를 사용하지 않는다. 교체 시 API에 두 ID를 일시 허용 → 이전 워커 drain → 새 ID 기동 → 이전 자격 제거 순서다. `.env`와 `worker.env`는 `chmod 600`으로 보호한다. 오래된 Pretendard 등 폰트 경로 override는 제거해 검증된 이미지 기본값을 사용한다.

워커 한 개의 시작 예산은 CPU 1개/메모리 512MiB, swap 없음, 동시 작업 1개, 인코딩/디코딩 스레드 1/2다. 작업 공간 8GiB, 준비 산출물 512MiB, 입력 파일 2GiB 한도는 유지한다. API/Caddy/OS/다른 서비스의 여유를 별도로 남긴다. **배포 전에 [8.6](#media-capacity)의 실제 호스트 용량 점검을 통과해야 한다.** 제한값은 실제 서버의 여유나 최악 입력의 완료를 보장하지 않는다.

<a id="media-cpu"></a>
### 8.2 GPU 없는 한 서버 — 현재 VPS 기본값

VPS `/srv/postpilot-prod`에 root의 prod/colocated/remote/nvidia Compose 파일, `deploy/backend-rollout.sh`, `deploy/media_rollout.py`, `deploy/media_preflight.py`, `deploy/media/worker.env.example`을 저장소와 같은 상대 경로로 둔다. 배포 워크플로가 이 파일들을 동기화한다. 첫 수동 배포 전에는 같은 revision의 파일을 먼저 복사한다. 공유 Caddy는 `/srv/edge`에 그대로 둔다.

API의 기존 `.env`는 유지한다. GitHub Actions는 `--init-worker`를 명시해, `worker.env`가 없는 최초 colocated CPU 배포에서만 8.1의 기본 워커 설정과 무작위 토큰을 만든다. API credential 목록에 새 워커를 추가하며 기존 항목·다른 env 값은 보존한다. 이미 `worker.env`가 있으면 해당 설정을 그대로 사용한다. API와 워커는 비공개 `media` bridge로 연결되며 `http://api:9000`은 컨테이너 내부 주소다. 워커 포트는 공개하지 않고 Caddy는 API의 8080으로만 연결한다. 기존 `data/`와 소유권을 보존한다.

**기존 VPS의 첫 자동 전환:** 배포 workflow가 `sh deploy/backend-rollout.sh --init-worker`를 실행한다. `worker.env`를 수동 생성하거나 GitHub Secret에 워커 토큰을 추가할 필요가 없다. 프로젝트별 ID와 32바이트 무작위 토큰을 생성하고 `.env`/`worker.env`에 0600 권한으로 저장한다. 원본 설정은 `.deploy/worker-init-before`에 보관한다. API credential 기록 직후 배포가 중단돼도 같은 토큰으로 재시도한다.

지원 버전으로 되돌릴 수 없는 첫 전환에서는 컨테이너 교체 전에 기존 bind-mounted SQLite DB의 일관된 백업을 `.deploy/worker-init-before/database.sqlite3`에 만든다. [Python SQLite backup API](https://docs.python.org/3/library/sqlite3.html#sqlite3.Connection.backup)를 사용해 원본을 read-only로 열며 WAL에 commit된 내용도 포함한다. 60초 제한·파일 접근·용량·무결성 검사 실패 시 기존 서비스는 교체하지 않는다. 새 설치라 DB 파일이 아직 없으면 백업할 데이터가 없다고 기록한다. 백업은 해당 시점의 사본이며 이후 쓰기까지 포함하는 실시간 복제나 자동 DB 복구가 아니다.

초기화 기록이 있고 아직 `last-good`가 없는 이 첫 CPU 전환에만 구버전 rollback 제한을 넘어 앞으로 배포한다. 첫 성공 이후에는 원래의 계약/rollback 검사를 적용한다. 첫 배포가 실패하면 같은 설정으로 재실행할 수 있으며, 마이그레이션 이후 구 API를 다시 부팅하지 않는다. `--init-worker`는 remote/GPU 구성을 추측하지 않고, 이미 정상 배포한 스택에서 지워진 worker.env도 새 토큰으로 덮어 만들지 않는다. 후자는 `.deploy/last-good`의 비공개 설정으로 복구한다.

수동 실행도 같은 경로를 사용할 수 있다. 서버 용량 점검은 8.6을 먼저 따른다.

```bash
cd /srv/postpilot-prod
IMAGE_TAG='<api-sha>' MEDIA_WORKER_IMAGE_TAG='<worker-sha>' \
  API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh --init-worker
```

**직접 worker.env를 준비한 첫 전환:** 8.1의 값을 채운 뒤 신규 요청을 잠시 중지하고 기존 처리 완료 → 일관된 SQLite 백업과 이전 설정 보관 → 아래 점검/배포 순서를 사용한다. 이 경로는 기존의 명시적인 `--bootstrap` 절차다. 이전 API에 `rollback-safe=1` 라벨이 없으면 구버전 이미지로 돌아갈 수 없다. `--init-worker`는 `--check`, `--drain`, `--rollback`과 함께 사용하지 않는다.

```bash
cd /srv/postpilot-prod
chmod 600 .env worker.env
IMAGE_TAG='<api-sha>' MEDIA_WORKER_IMAGE_TAG='<worker-sha>' \
  API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh --check --bootstrap
IMAGE_TAG='<api-sha>' MEDIA_WORKER_IMAGE_TAG='<worker-sha>' \
  API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh --bootstrap

# 이후 일상 업데이트: --bootstrap 생략
IMAGE_TAG='<api-sha>' MEDIA_WORKER_IMAGE_TAG='<worker-sha>' \
  API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh
# 상태 / 로그 / 워커 실제 실행과 API 인증 확인
cpu_compose() { docker compose --env-file .env --env-file worker.env \
  -f docker-compose.prod.yml -f docker-compose.media.colocated.yml "$@"; }
cpu_compose ps
cpu_compose logs --tail 80 api media-worker
cpu_compose exec -T media-worker /media-worker health
curl --fail 'https://<api-domain>/health'
# drain과 롤백은 8.5의 공통 명령 사용
```

`--check`는 이미지를 pull하고 일회용 검사 컨테이너를 실행하지만 운영 서비스를 중지하거나 입력 태그를 영구 반영하지 않는다. 첫 전환 설정은 `.deploy/bootstrap-before`에 보관한다. 구 API로 롤백할 수 없는 첫 배포가 실패하면 새 버전을 유지하고 로그의 원인을 수정한다. 오래된 DB 복원으로 이미지 롤백을 강제하지 않는다. 이후 지원 버전끼리는 자동 롤백이 동작한다.

<a id="media-single-gpu"></a>
### 8.3 NVIDIA GPU가 있는 한 서버

**CPU 실행만 원하면 8.2를 그대로 사용한다. GPU 드라이버도 필요 없다.** GPU 장치 노출/진단을 준비할 때만 8.7의 1회성 드라이버·toolkit 설정을 먼저 완료한다. API/SQLite/Caddy 위치, 네트워크, 데이터 디렉터리는 8.2와 같다.

8.1의 전체 `worker.env` 중 다음 값을 바꾸고, API `.env`의 credential ID도 같은 것으로 바꾼다. 나머지 env와 자원 한도는 유지한다. `MEDIA_ACCEL=cpu`는 의도적이다.

```dotenv
MEDIA_WORKER_IMAGE_TAG=<candidate-sha>
MEDIA_WORKER_VARIANT=nvidia-candidate
MEDIA_WORKER_ID=prod-gpu-1
MEDIA_WORKER_TOKEN=<new-token>
MEDIA_API_URL=http://api:9000
MEDIA_ACCEL=cpu
```

API `.env`는 `MEDIA_TOPOLOGY=colocated`, `MEDIA_WORKER_CREDENTIALS={"prod-gpu-1":"<new-token>"}`이다. controller가 `docker-compose.media.nvidia.yml`을 추가해 후보 이미지와 GPU 1개를 선택한다. **후보 태그는 `worker.env`에서 직접 선택한다.** API CI가 넘긴 CPU 워커 SHA로 덮어쓰지 않는다. 공개 이미지: `ghcr.io/hetarho/postpilot-media-worker-nvidia:<candidate-sha>`; API 이미지는 기존 `postpilot-api:<api-sha>`다.

```bash
cd /srv/postpilot-prod
chmod 600 .env worker.env
# 신규 스택/첫 구버전 전환에서만 8.2의 유지보수 후 --bootstrap 추가
IMAGE_TAG='<api-sha>' API_ORIGIN='https://<api-domain>' \
  sh deploy/backend-rollout.sh --check
IMAGE_TAG='<api-sha>' API_ORIGIN='https://<api-domain>' \
  sh deploy/backend-rollout.sh
# 이후 업데이트도 후보 태그를 worker.env에 먼저 기록한 뒤 같은 명령 사용
nvidia_compose() { docker compose --env-file .env --env-file worker.env \
  -f docker-compose.prod.yml -f docker-compose.media.colocated.yml \
  -f docker-compose.media.nvidia.yml "$@"; }
nvidia_compose ps
nvidia_compose logs --tail 80 api media-worker
nvidia_compose exec -T media-worker /media-worker health
curl --fail 'https://<api-domain>/health'
```

운영 워커의 상태는 계속 `Profile: cpu`다. GPU가 없거나 Docker가 장치를 노출할 수 없으면 **GPU override를 선택한 컨테이너의 기동이 실패**한다. 드라이버 복구 또는 8.5의 이전 CPU 구성 롤백을 사용한다. GPU 호스트용 이미지에서 CPU 합성·resvg·폰트는 기존 바이너리를 그대로 사용한다.

<a id="media-remote-gpu"></a>
### 8.4 API VPS 한 대 + 별도 NVIDIA PC 한 대 — 나중에 이동할 때

VPS에는 API/SQLite/Caddy만, PC에는 `media-worker`만 배치한다. PC는 API의 비공개 HTTPS와 오브젝트 저장소로 **먼저 나가는 연결**을 만든다. PC에 공개 수신 포트, 공유 DB/볼륨, API 비밀키가 필요 없다. API 이미지가 준비·렌더를 대신 수행하는 자동 fallback도 없다.

**1회성 네트워크 설정:** 양쪽 Linux 호스트에 Tailscale을 설치·등록하고 tailnet HTTPS를 활성화한다. tailnet 접근 정책에서 이 PC의 태그/신원에 VPS의 TCP 9443만 허용한다. 다른 tailnet 기기에 광범위하게 열어두지 않는다. VPS의 기존 Serve 설정과 충돌하지 않는 포트를 고르고 아래 규칙을 추가한다. 선택한 포트를 바꾸면 URL과 정책도 함께 바꾼다.

```bash
# VPS: remote override가 127.0.0.1:9000을 바인딩한 뒤 사용
sudo tailscale serve --bg --https=9443 http://127.0.0.1:9000
tailscale serve status
```

[비공개 Serve](https://tailscale.com/docs/features/tailscale-serve)와 [명령 문서](https://tailscale.com/docs/reference/tailscale-cli/serve)를 따른다. public Funnel을 사용하지 않는다. VPS의 공개 방화벽/Caddy에 9000/9443을 추가하지 않는다. PC에서는 기존 방화벽을 유지하고 Tailscale의 연결 및 HTTPS 스토리지 송수신을 허용한다. `MEDIA_STORAGE_ENDPOINT`가 localhost/Compose의 `minio` 이름이면 원격에서 접근할 수 없다. R2처럼 양쪽에서 접근 가능한 endpoint를 API에 설정하고 서명 URL의 hostname을 중간 프록시에서 바꾸지 않는다. 워커는 짧은 수명의 권한만 받는다.

**VPS 설정:** 기존 `.env` 전체를 보존하되 아래 항목을 바꾼다. `worker.env`는 8.1 전체 예제에 아래 PC 변경분을 적용한 사본이다. VPS의 호환성 점검용 후보 이미지 pull/CPU 검사는 GPU 없이 실행된다.

```dotenv
IMAGE_TAG=<api-sha>
MEDIA_TOPOLOGY=remote
MEDIA_WORKER_CREDENTIALS={"prod-gpu-remote-1":"<new-token>"}
MEDIA_STORAGE_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
```

**PC와 VPS의 `worker.env` 변경분:** 나머지는 8.1 그대로다. PC의 이미지 태그는 독립적으로 선택한다.

```dotenv
MEDIA_WORKER_IMAGE_TAG=<candidate-sha>
MEDIA_WORKER_VARIANT=nvidia-candidate
MEDIA_API_URL=https://<vps>.<tailnet>.ts.net:9443
MEDIA_WORKER_ID=prod-gpu-remote-1
MEDIA_WORKER_TOKEN=<new-token>
MEDIA_ACCEL=cpu
```

**파일 배치:** PC에 `/srv/postpilot-media`를 만들고 `deploy/media/docker-compose.yml` → `docker-compose.yml`, `deploy/media/docker-compose.nvidia.yml` → `docker-compose.nvidia.yml`로 복사한다. `deploy/media/rollout.sh`, `deploy/media_rollout.py`, `deploy/media_preflight.py`는 저장소의 상대 경로를 유지한다. 그 디렉터리에는 PC의 `worker.env`만 둔다. GPU 진단 파일/fixture는 별도 디렉터리를 쓴다. 8.7의 드라이버/toolkit과 8.6의 용량 점검(`--api-extra-memory 0`)을 이 PC에서 완료한다.

```bash
# VPS: 로컬 워커를 멈춘 뒤 새 배치를 적용하고 API 상태 확인
cd /srv/postpilot-prod
IMAGE_TAG='<api-sha>' API_ORIGIN='https://<api-domain>' \
  sh deploy/backend-rollout.sh --check
IMAGE_TAG='<api-sha>' API_ORIGIN='https://<api-domain>' \
  sh deploy/backend-rollout.sh
remote_api() { docker compose --env-file .env --env-file worker.env \
  -f docker-compose.prod.yml -f docker-compose.media.remote.yml "$@"; }
remote_api ps
remote_api logs --tail 80 api
curl --fail 'https://<api-domain>/health'

# PC: 첫 활성화. API와 연결되는 status는 아직 사용자 작업을 claim하지 않는다.
cd /srv/postpilot-media
chmod 600 worker.env
remote_worker() { docker compose --env-file worker.env \
  -f docker-compose.yml -f docker-compose.nvidia.yml "$@"; }
remote_worker pull media-worker
remote_worker run --rm --no-deps -T media-worker status
MEDIA_WORKER_IMAGE_TAG='<candidate-sha>' sh deploy/media/rollout.sh --check --bootstrap
MEDIA_WORKER_IMAGE_TAG='<candidate-sha>' sh deploy/media/rollout.sh --bootstrap
remote_worker ps
remote_worker logs --tail 80 media-worker
remote_worker exec -T media-worker /media-worker health
# 이후 PC에서 독립 업데이트 / 중지 / 롤백
MEDIA_WORKER_IMAGE_TAG='<next-candidate-sha>' sh deploy/media/rollout.sh
sh deploy/media/rollout.sh --drain
sh deploy/media/rollout.sh --rollback
```

VPS 배포 controller가 같은 Compose 프로젝트의 이전 로컬 워커를 먼저 drain한다. 중지된 이전 컨테이너는 실행 중인 작업이 없는 것을 확인한 후에만 정리하며 `--remove-orphans`를 사용하지 않는다. PC의 `status`와 운영자 소유 테스트 프로젝트의 렌더/삭제를 확인한 뒤 전환을 완료한다. PC 업데이트가 검증되면 VPS `worker.env`에도 그 후보 SHA를 기록한다. API의 다음 배포는 PC의 켜짐 여부나 동시 릴리스를 요구하지 않는다.

PC가 꺼지면 작업은 유한한 대기/재시도 예산(기본 대기 30분, 전체 stage 2시간, 최대 3회) 안에서 기다리거나 실패한다. API/웹은 계속 응답한다. CPU 보조 워커를 원하면 VPS에 **별도 Compose 프로젝트의 standalone CPU 워커**, 고유 ID/토큰, 비공개 HTTPS 주소와 충분한 자원을 명시적으로 배포하고 API에 그 ID를 허용해야 한다. 해당 작업의 프로필·렌더러·에셋 계약이 맞을 때만 받을 수 있다. 엄격한 GPU 작업을 CPU가 몰래 대신 처리하는 정책은 없다.

<a id="media-recovery"></a>
### 8.5 중지·롤백·버전 호환성

VPS에서 다음 명령은 세 배치 모두 동일하다. PC의 워커 전용 명령은 8.4에 있다.

```bash
cd /srv/postpilot-prod
API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh --drain
API_ORIGIN='https://<api-domain>' sh deploy/backend-rollout.sh --rollback
```

SIGTERM은 새 claim을 중단한다. 기본 drain 30초 + 최종 보고/프로세스 회수 15초이며 `MEDIA_STOP_TIMEOUT`은 drain보다 최소 15초 길어야 한다. 끝내지 못한 lease는 API가 복구하며 결제된 모델 호출을 재실행하지 않는다. `--drain`은 중지 명령이며, 재개는 선택한 이미지의 정상 rollout을 다시 실행한다.

controller는 protocol/renderer/assets/asset-input 해시, 실제 CPU 도구·폰트, private 인증을 확인한 뒤 교체한다. 실패 시 `.deploy/last-good`의 **이미지, env, Compose 파일**을 함께 복원하고 건강 상태를 다시 검사한다. 수동 `--rollback`은 `.deploy/previous`를 사용한다. 첫 기동에는 이전 지원 스냅샷이 없을 수 있다. CPU↔GPU 후보 전환도 스냅샷으로 되돌릴 수 있지만 원격 PC를 자동으로 켜거나 설정을 바꾸지는 않는다. `MEDIA_TOPOLOGY` 변경을 되돌릴 때 PC를 먼저 drain하여 중복 신원을 피한다.

`.deploy`에는 토큰이 있으므로 비공개로 유지한다. 현재/직전 이미지와 컨테이너가 참조하는 이미지는 보존하며 API/CPU 워커/NVIDIA 후보 저장소의 오래된 미사용 태그만 정리한다. SQLite 마이그레이션은 forward-only다. `rollback-safe=1` 이전 API 부팅은 거부한다. 자동 롤백 자체도 실패하면 `.deploy` 스냅샷과 시작 로그로 복구한다.

<a id="media-capacity"></a>
### 8.6 CPU 릴리스 검증과 실제 호스트 용량 점검

현재 VPS는 **colocated CPU / 워커 1개 작업**이 기본이다. 로컬 테스트 통과는 현재 VPS의 여유 메모리 측정이나 배포 완료를 뜻하지 않는다. API에는 편집안 검증, 미리보기와 브라우저 자막 에셋용 FFmpeg/resvg/폰트가 계속 필요하다. 서버 영상의 준비·최종 렌더는 같은 서버에서도 전용 워커가 처리한다.

배포 전 **실제 Linux Docker 호스트**에서 실행한다. `--work-root`는 워커 볼륨이 놓이는 파일시스템의 기존 디렉터리로 바꾼다. Docker Desktop을 쓰는 Mac의 메모리를 VPS 용량으로 해석하지 않는다.

```bash
python3 deploy/media_preflight.py \
  --work-root /var/lib/docker \
  --worker-memory 512m --worker-cpus 1 \
  --api-extra-memory 256m --reserve-memory 256m \
  --workspace 8g --reserve-disk 2g
```

`worker.env`의 `MEDIA_WORKER_MEMORY`/`MEDIA_WORKER_CPUS`와 같은 값을 넣는다. 메모리는 전체 RAM이 아니라 **현재 MemAvailable**과 cgroup 잔여량 중 작은 값이다. 이미 실행 중인 DB/API/Caddy 등은 현재 사용량으로 반영되며, API 추가 여유와 다른 서비스의 증가분도 별도로 남긴다. 원격 워커 전용 호스트에서는 `--api-extra-memory 0`을 사용한다. 기본 디스크 요구량은 제품의 작업 공간 상한 8GiB + 여유 2GiB다. 실패(exit 1)하면 서버 증설, 동시 워크로드 축소 또는 별도 워커 호스트를 결정한 뒤 다시 점검한다. 이 일회성 검사는 미래 부하와 처리 속도를 보장하지 않는다.

개발 머신에서 재현하는 명령이다. 자원 측정 smoke에는 cgroup v2의 `memory.peak`를 제공하는 Linux Docker 환경이 필요하다. 지표를 읽을 수 없으면 검증은 실패한다.

```bash
pnpm smoke:media-worker
pnpm smoke:media-release
```

첫 명령은 동일 머신·동일 CPU 프로필의 직접 실행과 워커 실행을 비교한다. 세 화면 비율, 원본 오디오, 전환과 프레임별 자막을 포함하며 기존 파일 해시를 다른 아키텍처에 맞춰 갱신하지 않는다. 두 번째 명령은 실제 API 자식 프로세스, 독립 워커 컨테이너, 비공개 MinIO를 사용한다. 모델 응답만 명시적인 합성 fixture다. 같은 네트워크와 두 네트워크 사이의 제한된 릴레이 환경을 모두 실행하며 API 재시작, 워커 강제 종료/회수, 취소·중복 완료·삭제 재시도를 검증한다.

실행 기본 한도는 API 256MiB/1CPU + 워커 512MiB/1CPU, 같은 호스트의 다른 서비스용 예비 256MiB로 합계 1GiB다. MinIO(256MiB/0.5CPU)와 원격 테스트 릴레이(64MiB/0.25CPU)는 실제 배포의 외부 R2/네트워크를 대신하는 **별도 테스트 인프라**이므로 실행 한도 밖에 두고 보고서에 구분한다. 합산 메모리 피크는 두 컨테이너 `memory.peak`의 합으로 계산한 보수적인 상한이며, 디스크 피크는 주기적 표본의 최대값이다. 큰 실제 사용자 입력의 최악 조건 벤치마크와 같지 않다.

2026-09-25 개발 머신의 Linux/arm64 컨테이너에서 얻은 최종 결과다. 16초짜리 합성 원본 1개로 15초 결과를 만들고, 별도의 출력 동일성 검사는 세 비율의 오디오·전환·자막 사례를 비교했다. GPU와 실제 VPS에서는 실행하지 않았다.

| 로컬 배치 | API+워커 메모리 피크 상한 | 표본 디스크 피크 합계 | 프로젝트 RPC 최대 | 준비(다운로드+처리) | 렌더(다운로드+처리) | 준비/결과 업로드 구간 |
| --- | --- | --- | --- | --- | --- | --- |
| colocated | 544.4MiB | 16.4MiB | 37ms | 1,371ms | 79,796ms | 13ms / 15ms |
| remote 네트워크 | 522.3MiB | 15.1MiB | 12ms | 1,183ms | 78,523ms | 17ms / 10ms |

렌더 시간은 작업 회수 대기와 분리해 기록한 마지막 실제 처리 구간이다. 최초 렌더의 강제 종료·회수·API 재시작을 포함한 전체 시간은 각각 99,779ms/94,258ms였다. 업로드 구간에는 슬롯 예약 RPC가 포함된다. 이 작은 합성 입력의 값으로 실제 영상 최대 크기나 GPU 속도를 추정하지 않는다.

`MEDIA_RELEASE_REPORT`와 `MEDIA_RESOURCE_REPORT`가 검증 범위, RPC 응답 시간, 준비·렌더·업로드 구간, 자원 사용량을 기록한다. OOM, 예산 초과, 시간 초과, 누락된 자원 보고는 실패다. 필요한 경우 `python3 scripts/media-release.py --help`의 명시적인 한도를 바꿔 더 큰 호스트용 결과를 별도로 남긴다.

**나중에 운영자가 staging/VPS에서 실행할 선택적 smoke:** 저장소를 별도 체크아웃하고, 위 사전 점검 후 테스트 인프라와 이미지 빌드에 필요한 추가 용량까지 확보한 경우에만 `python3 scripts/media-release.py --layout colocated`를 실행한다. 운영 `.env`를 복사하거나 production Compose를 이 명령과 합치지 않는다. 도구는 매번 `postpilot-release-<random>` 컨테이너·네트워크·워커 볼륨과 일회용 MinIO `release` 버킷을 만들고, API의 SQLite/임시 원본은 해당 테스트 컨테이너에만 둔다. 제어 포트는 `127.0.0.1`의 임의 포트이고 내부 워커 포트는 테스트 네트워크의 9002다. `finally` 정리는 자신이 생성한 이름만 제거하며 기존 프로젝트·DB·원본·볼륨을 열지 않는다. 실행 중 호스트 자체가 종료되면 해당 실행의 이름과 `postpilot.disposable-media-release` 라벨을 확인해 그 일회용 리소스만 정리한다. 이번 작업에서는 실제 VPS에서 이 절차를 실행하지 않는다.

<a id="media-gpu-diagnostics"></a>
### 8.7 NVIDIA 1회성 설치와 격리 진단 — 나중에 GPU 호스트에서

이 절차는 **GPU 가속의 운영 활성화가 아니다.** 후보는 최종 H.264 인코딩에 NVENC를 사용해 비교한다. CPU 합성, xfade/overlay, resvg 자막, 기존 중간 표현과 오디오 처리는 그대로 남는다. GPU 디코딩은 별도 짧은 probe이고 benchmark 입력 디코딩은 CPU다. `scale_cuda`는 CUDA 컴파일러를 포함하지 않은 이 이미지에서는 미검증/미포함이다.

**호스트 준비(최초 1회):** [NVIDIA 지원 GPU/드라이버 안내](https://docs.nvidia.com/video-technologies/video-codec-sdk/13.1/ffmpeg-with-nvidia-gpu/index.html)를 통해 실제 모델의 H.264 NVENC 지원을 확인하고 배포판의 공식 방법으로 드라이버를 설치한다. 고정한 [nv-codec-headers n13.0.19.0](https://github.com/FFmpeg/nv-codec-headers/blob/n13.0.19.0/README)의 Linux 최소 드라이버는 **570.0**이다. 버전 숫자만 충족해도 장치 지원과 실제 실행은 별도로 확인해야 한다. 호스트에서 `nvidia-smi`가 먼저 성공해야 한다.

Ubuntu/Debian 계열의 [공식 Container Toolkit 설치법](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)을 따른 예다(확인일 2026-09-25, 1.20.1-1). 다른 배포판에는 해당 문서의 패키지 관리 절차를 적용한다. Docker 재시작은 서비스에 영향을 주므로 운영자가 정한 설치 시간에 실행한다.

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl gnupg2
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
  sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
  sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt-get update
NVIDIA_CONTAINER_TOOLKIT_VERSION=1.20.1-1
sudo apt-get install -y \
  nvidia-container-toolkit=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
  nvidia-container-toolkit-base=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
  libnvidia-container-tools=${NVIDIA_CONTAINER_TOOLKIT_VERSION} \
  libnvidia-container1=${NVIDIA_CONTAINER_TOOLKIT_VERSION}
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
nvidia-smi
```

후보 이미지는 glibc Debian bookworm 기반이다. 기존 정적 CPU 도구는 `/usr/local/bin`, 별도 FFmpeg NVENC 도구는 `/opt/nvidia/bin`에 둔다. Debian digest/날짜 고정 패키지 저장소, FFmpeg 9.0.1과 headers의 SHA-256, 빌드 설정·패키지 목록·소스·라이선스는 `backend/build/nvidia-tools.sh`와 이미지의 `/opt/nvidia/share`에 기록한다. CUDA SDK/NPP를 호스트에 설치하는 절차는 필요하지 않다. [NVIDIA 컨테이너 capability](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/docker-specialized.html)의 `compute,video,utility`로 드라이버 라이브러리를 주입하며, [Compose GPU 예약](https://docs.docker.com/compose/how-tos/gpu-support/)으로 GPU 1개를 명시한다. CPU 기본 Compose에는 GPU 예약이 없다.

**같은 장비에서 합성 fixture 생성:** 운영 워커가 없는 별도 저장소 체크아웃에서 해당 candidate SHA로 빌드한다. 기존 출력 동일성 smoke와 같은 CPU 2개/1GiB 예산을 사용하며 30분 제한과 추가 빌드 디스크 여유가 필요하다. 다음 테스트 전용 이미지에는 fixture 테스트 바이너리가 있고, 실제 배포 후보에는 없다.

```bash
# GPU Linux 호스트의 별도 체크아웃에서 실행; 실제 운영 env를 가져오지 않는다.
docker build -f backend/Dockerfile --target media-worker-nvidia-smoke \
  -t postpilot-nvidia-diagnostic:local .
sudo install -d -o 65532 -g 65532 /srv/postpilot-gpu-check
# /srv/postpilot-gpu-check/fixtures는 아직 없어야 한다.
docker run --rm --network none --cpus 2 --memory 1g --memory-swap 1g \
  --mount type=bind,src=/srv/postpilot-gpu-check,dst=/reports \
  -e CLIP_DIAGNOSTIC_FIXTURES=/reports/fixtures \
  --entrypoint /media.test postpilot-nvidia-diagnostic:local \
  '-test.run=^TestWorkerExecutionParity$' -test.v -test.timeout=30m
sudo install -d -o 65532 -g 65532 /srv/postpilot-gpu-check/reports
sudo cp deploy/media/docker-compose.diagnostics.yml /srv/postpilot-gpu-check/
# 이 파일에는 태그만 넣는다. worker.env/API .env를 사용하지 않는다.
printf 'MEDIA_WORKER_IMAGE_TAG=%s\n' '<candidate-sha>' | \
  sudo tee /srv/postpilot-gpu-check/diagnostic.env
```

fixture는 세 비율의 움직임·전환·원본 오디오·프레임별 자막을 담은 합성 MP4 네 개와 `manifest.json`이다. 직접 CPU 렌더와 워커 출력의 바이트/프레임 동일성을 확인하면서 내보낸다. manifest의 `CPUCompositionAndEncodeMS`는 fixture 생성 때의 합성+CPU 인코딩 시간이며 별도 재인코딩 시간과 합산해 운영 속도 향상으로 해석하지 않는다.

**오프라인 장치/인코더 확인 및 비교:** 다음 컨테이너는 `network_mode: none`, API credential 없음, 입력 read-only, 별도 report 폴더, CPU 1개/메모리 1GiB/swap 없음으로 실행한다. 프로세스당 최대 3분, 전체 15분, 최대 fixture 6개/파일 128MiB/영상 90초/1080p 면적, 결과 MP4 각 128MiB 한도다. 한도가 부족하면 진단이 실패하며 운영 설정을 자동 변경하지 않는다. report/fixture를 위한 여유 디스크 4GiB 이상을 남긴다. 결과 디렉터리는 매번 새 이름이어야 한다.

```bash
cd /srv/postpilot-gpu-check
diag() { docker compose --env-file diagnostic.env \
  -f docker-compose.diagnostics.yml "$@"; }
diag config --quiet
diag pull diagnostic
# 컨테이너 내부 장치 주입 확인
# nvidia-smi의 성공과 실제 NVENC encode 성공을 모두 확인해야 한다.
diag run --rm --entrypoint nvidia-smi diagnostic
diag run --rm diagnostic gpu-probe --output /reports/probe-01
diag run --rm diagnostic benchmark \
  --manifest /fixtures/manifest.json --output /reports/compare-01
sudo cat reports/probe-01/report.json
sudo cat reports/compare-01/report.json
```

`gpu-probe`는 device/UUID/driver/VRAM 관찰값, 실제 NVENC 3프레임 인코딩, 별도 CUVID 디코딩 결과를 기록한다. 장치/인코더가 없으면 JSON 실패 보고서를 남기고 exit 1이다. Compose가 GPU 장치를 예약하지 못해 프로세스 자체가 시작하지 않으면 보고서는 생기지 않으므로 먼저 Docker/드라이버 설정을 확인한다. CUVID 실패는 독립 결과이며 NVENC 지원과 혼동하지 않는다.

**GPU 없는 개발 장비의 검사:** 위 GPU Compose 대신 아래처럼 장치 옵션 없이 candidate smoke를 실행한다. 첫 명령은 CPU 실행/잘못된 옵션/보고서 덮어쓰기 거부/장치 누락의 실패 보고를 테스트한다. 두 번째는 이미 생성한 fixture를 CPU만 재인코딩한다. `/reports/cpu-01`은 아직 없어야 한다.

```bash
docker run --rm --network none --cpus 1 --memory 512m --memory-swap 512m \
  postpilot-nvidia-diagnostic:local
docker run --rm --network none --cpus 1 --memory 1g --memory-swap 1g \
  --mount type=bind,src=/srv/postpilot-gpu-check/fixtures,dst=/fixtures,readonly \
  --mount type=bind,src=/srv/postpilot-gpu-check/reports,dst=/reports \
  --entrypoint /media-worker postpilot-nvidia-diagnostic:local \
  benchmark --cpu-only --manifest /fixtures/manifest.json --output /reports/cpu-01
```

`report.json`은 profile/실행 범위, OS/아키텍처, CPU 도구·폰트·에셋 해시, NVIDIA FFmpeg 버전/해시, 장치/드라이버, 정확한 인코더 인자, 입력 해시, CPU/GPU 인코딩 ms, 파일 크기/해시, ffprobe 스트림/시간 정보, 8개 시점 PNG 해시·평균 RGB 절대 차이, 디코딩한 오디오 동일 여부, cgroup CPU/메모리 상한·피크, 남긴 파일 총 바이트를 담는다. PNG/MP4/WAV도 비교용으로 남는다. VRAM은 probe 시점 관찰값이며 peak 측정이 아니다. 메모리 peak는 해당 컨테이너 전체, 파일 바이트는 report.json을 쓰기 직전 산출 파일의 합계다. cgroup 지표가 없는 호스트에서는 빠진 값을 미측정으로 간주하고, 실제 승인용 측정은 cgroup v2 지표가 제공되는 환경에서 수행한다.

CPU는 x264 `veryfast/CRF 20`, GPU 후보는 NVENC `p4/hq/vbr/CQ 20/b:v 0`이다. **CRF 20과 CQ 20은 같은 품질이라는 뜻이 아니다.** 이미 CPU로 합성·인코딩한 MP4를 각각 다시 인코딩하므로 이 비교는 최종 인코더 차이를 보는 진단이며, 무손실 master 기반 비교나 전체 렌더 성능 검증을 대신하지 않는다. 오디오는 이미 검증된 fixture의 AAC를 copy하고 decoded PCM/스트림 길이로 변화 여부를 확인한다.

같은 GPU 장비·이미지·fixture 해시·자원 한도로 반복하고, 자막 테두리/움직임/전환 구간을 직접 확대 비교하며 오디오와 A/V 타이밍을 확인한다. 임의의 perceptual 합격 수치나 속도 향상률을 코드에 넣지 않았다. 이후 CLIP-163 단계에서 무손실 입력과 실제 전체 파이프라인, 품질 기준, GPU 장애 처리까지 검증·승인하고 production 프로필을 구현해야 `auto`가 GPU를 선택할 수 있다. 모든 진단의 `ProductionApproved`는 항상 `false`다.

### 8.8 이번 검증 범위와 이후 운영 작업

2026-09-25 Linux/arm64 CPU 컨테이너에서 후보 이미지 빌드, CPU 실제 encode/decode와 오프라인 실행, 장치 누락 실패 보고, 세 배치의 Compose 렌더링/롤백 계약을 검증했다. 세 비율 합성 fixture 네 개를 후보 이미지의 CPU 경로로 내보내고 기존 워커/직접 렌더의 파일 해시와 각 16개 프레임이 일치하는 것을 확인했다(636.98초, 테스트 컨테이너 CPU 2개/1GiB, 관찰 메모리 peak 약 690.9MiB, OOM 0). 같은 복잡한 fixture의 512MiB/1CPU 시도에서는 최종 인코더가 process_signal로 종료되어 그 조건을 통과한 것으로 기록하지 않았다. 이는 8.6의 작은 분리 프로세스 fixture와 다른 부하이며, 512MiB를 모든 입력에 충분한 운영 용량으로 해석하지 않는다. **실제 NVIDIA 장치의 encode/decode, GPU 시간/화질/VRAM, Linux/amd64 GPU 하드웨어 동작은 미검증이다.** CI의 수동 후보 게시도 하드웨어 검증으로 간주하지 않는다.

현재 VPS 재배포, 친구 PC 접근, 드라이버/Tailscale 설치, 실물 GPU benchmark와 실제 워커 이동은 이번 저장소 작업에서 실행하지 않았다. 나중에 운영자가 이 문서의 해당 환경 절차를 실행한다. 최초 설치가 완료된 호스트 사이의 배치 변경은 env·게시 이미지·Compose 선택으로 처리하며, 다른 OS/장치의 설치 요구를 env만으로 해결하지 않는다.
