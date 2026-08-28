# Policy — Photos and object storage

Canonical rules that are **currently true** in the code. Source: [plan/02](../plan/02.post-drafting-and-list.md),
backend built by job 03. The browser half of the pipeline (extensions, HEIC decode, resize, encode) is job 05 and
will extend this document.

## The API never touches photo bytes

This is invariant **[I6]**, and it is what keeps the Go image CGO-free and distroless:

- No RPC accepts or returns image data. `CreateUpload` hands back a **presigned PUT on the storage host**, and the
  browser uploads directly.
- The server never decodes an image. `width` and `height` come from the client because only it decoded the file;
  `bytes` comes from the server's own HEAD, because that is the part the server can check.
- `maxRequestBytes` on the RPC server stays at 256 KiB precisely because the largest thing an RPC carries is a memo.

## The upload handshake

```
CreateUpload(post_slug, filename)  → upload_id, presigned PUT, content_type, expires_at
PUT directly to storage            ← the API is not in this path
ConfirmUpload(upload_id, w, h)     → the server HEADs the object, then records the photo
```

- The image id is minted at `CreateUpload`, not at confirm, because the object key contains it: the browser has to
  PUT to the final key, and the server has to find that object again from an `upload_id` alone after a restart.
- **The row is written after the presign succeeds.** A row with no usable URL would only be swept later for nothing.
- **Confirm is one transaction**: the photo row is written and the upload row dropped together. They must not be
  separable — an upload row that outlived its move still names the live photo's object, and the sweep would then
  delete the bytes while the photo row went on looking healthy.
- **Confirm is idempotent.** A client that never saw the response retries; the retry returns the photo rather than a
  primary-key failure.
- A confirm whose object is not in storage is `FailedPrecondition`, and the upload row **stays** so the client can
  retry the PUT with the same id.

## What the server refuses

- `Content-Type: image/jpeg` is **part of the presigned signature**. A PUT with any other value fails as a signature
  mismatch, and one with none fails as an unsigned header. The value is returned alongside the URL so the client can
  send exactly it.
- Dimensions must be positive and at most 20000 px. Object size must be positive and at most **10 MiB**; a rejected
  object is deleted immediately rather than left for the sweep. The browser's own 25 MB cap is advice — a presigned
  PUT is a URL an authenticated client can use however it likes, so the limit is enforced where it can be trusted.
- A filename already held by a **confirmed** photo is `AlreadyExists`: the filename is how the model and the
  exporters address a photo, so two cannot share one within a post.
- A filename held by a **pending** upload is not a conflict — that is the retry case. The pending upload is replaced
  (its row and object dropped) and a fresh id issued. Refusing would strand the user until the sweep ran.
- `UNIQUE(post_slug, filename)` on **both** `images` and `uploads` is what actually closes the race between two
  concurrent `CreateUpload` calls; the service's precheck only makes the common case a clean error.

## Reading a photo

- The bucket is **private**. Nothing is readable without a presigned URL, and one is issued only after the caller's
  ownership of the post is checked (PRD F-5, §7).
- View URLs are minted **fresh on every `GetPost`** and never stored. A persisted URL would either be long-lived — a
  durable capability to a private object — or already expired.
- The storage key never crosses the wire. A client addresses a photo by id.

## Keys and cleanup

- Key shape: `posts/{slug}/{image_id}.jpg` (PRD §5). The id is random (16 bytes), so a key is not guessable from
  another user's and reveals nothing about how many photos exist.
- `DeleteImage` removes the **object first, then the row**. The reverse order could drop the only reference to an
  object whose delete then failed, leaving bytes nobody can name. The cost of this order is that a crash in between
  leaves a row pointing at a missing object — recoverable by retrying the delete, which is the better failure.
- The **orphan sweep** runs every `ORPHAN_SWEEP_INTERVAL` (default 24 h) and reclaims two different leaks:
  - uploads past `expires_at + ORPHAN_MIN_AGE` (1 h), with their objects — but never an object a photo row still
    references;
  - objects under `posts/` that no row names and that are older than `ORPHAN_MIN_AGE`. The age check is what keeps
    it from racing an upload in flight, which has an object and no row yet.
- The sweep **deletes nothing** if the listing or the key snapshot fails. A short read would look like "these
  objects are gone" and take live photos with it. For the same reason the referenced-key set is read as one
  transaction: a confirm landing between two separate queries could leave a live key in neither.
- The sweep does not run at boot — a restart loop would turn every crash into a full bucket listing, and nothing it
  collects is urgent.

## Configuration

| Value | Where | Note |
|---|---|---|
| `R2_ENDPOINT` | env, required | the endpoint the API calls |
| `R2_PUBLIC_ENDPOINT` | env, defaults to `R2_ENDPOINT` | the endpoint presigned URLs are signed against — the one the **browser** calls. Separate because a signature covers the Host header; in production both are the R2 endpoint and this is left unset |
| `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET` | env, required | |
| `ORPHAN_SWEEP_INTERVAL` | env | `24h`; PRD §9.5 leaves the cadence undecided |
| presign PUT/GET TTL | constants | 10 minutes each |
| `ORPHAN_MIN_AGE` | constant | 1 hour |
| max object size | constant | 10 MiB |

The server **refuses to start** without the four required values, naming the missing one. This is checked before the
listener, so a missing value keeps `/health` dark and the deploy rolls back — but it is deliberately *not* part of
`config.Load`, because `api adduser` must work on a fresh box before a bucket exists.

**Local development uses MinIO** (`docker-compose.yml`), not R2. Same S3 API, same four variables, so nothing in the
Go code knows the difference. Production R2 additionally needs a **CORS rule** allowing `PUT`/`GET`/`HEAD` from the
frontend origin — without it every server-side step reports success and only the browser's direct PUT fails
(`DEPLOY.md` §5).
