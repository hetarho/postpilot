# REVIEW clip-release-smoke-260914
> st:converted@260916 | scope:backend/cmd/api (release smoke), backend/internal/clip/ai/schemas | at:0c6a07f | base:ARCH@2

## summary
- the mandatory production-image gate has been failing since the observation contract moved to v2, so nothing has deployed: `eebe34c` and everything after it is built and refused
- the gate's own fixture is the only thing wrong; the contract change it fails against is correct

## findings
- F1 [o] P1 `backend/cmd/api/clip_release_fixtures_test.go:477`: bug: the synthetic observation answer still carries the v1 segment shape, while `AnalysisContractVersion` is `clip-observation-v2` and the closed schema now requires `action`, `motion`, `certainty` and `usability` on every segment ← `TestClipWriterInputRelease` is a `RUN` step of the production image (`Dockerfile:66`), so the first chunk is refused with `output_shape`, the writer burns its three corrections, and the build fails with `calls=4 want=21`; the image is never pushed and the rollout never runs →T158
- F2 [x] P2 `backend/cmd/api/clip_release_test.go:143` + `Dockerfile:65`: the release smoke runs only when `CLIP_RELEASE_SMOKE=1`, which is set only inside the image build, so ARCH-26's `go test ./...` skips the one check that gates deployment ← T145 passed every local gate and still broke the pipeline, and a developer cannot reproduce it: the harness wants the bundled faces at their absolute runtime paths and `prepare` wants ffmpeg. answered by ARCH-37: ARCH-26 never carries the smokes and building the image stage is the reproduction path a media task owes before done, so no code change follows

## notes
- F1's fix is fixture data only. The v2 coverage rule is already satisfied by the single `0..chunk_duration_ms` segment the fixture emits; only the four new required fields are missing
