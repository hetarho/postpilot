// Built only in an isolated diagnostic workspace; never imported by the product.
import React from "react";
import { AbsoluteFill, Img, useCurrentFrame, useVideoConfig } from "remotion";
import { Video } from "@remotion/media";
import { renderMediaOnWeb } from "@remotion/web-renderer";

const url = (file) => `/__browser-media-fixtures__/${file}`;
function Component({ fixture }) {
  const time = (useCurrentFrame() * 1000) / useVideoConfig().fps;
  return (
    <AbsoluteFill style={{ backgroundColor: "black" }}>
      <Video
        src={url(fixture.source.file)}
        muted
        style={{ width: "100%", height: "100%", objectFit: "cover" }}
      />
      {fixture.assets
        .filter((asset) => time >= asset.startMs && time < asset.endMs)
        .map((asset) => {
          const motion = window.browserMediaBenchmark.motion(asset, time);
          return (
            <Img
              key={asset.key}
              src={url(asset.file)}
              style={{
                position: "absolute",
                left: asset.x,
                top: asset.y + motion.dy,
                width: asset.width,
                height: asset.height,
                opacity: motion.opacity,
              }}
            />
          );
        })}
    </AbsoluteFill>
  );
}

window.browserMediaBenchmark.register("remotion-web-experiment", {
  async run(fixture) {
    if (!fixture.prepared)
      return {
        status: "missing-fixture",
        reason: "NATIVE_FIXTURE_NOT_PREPARED",
      };
    if (
      fixture.audio !== "none" ||
      fixture.assets.some((asset) => asset.representativeFrame) ||
      fixture.plan.cuts.length !== 1
    )
      return {
        status: "unsupported",
        reason: "EXPERIMENT_SUPPORTS_SINGLE_CUT_STATIC_INK_ONLY",
      };
    const dimensions =
      fixture.ratio === "vertical"
        ? [1080, 1920]
        : fixture.ratio === "horizontal"
          ? [1920, 1080]
          : [1080, 1080];
    const started = performance.now();
    let encodedFrames = 0;
    try {
      const rendered = await renderMediaOnWeb({
        composition: {
          id: fixture.id,
          component: Component,
          durationInFrames: (fixture.durationMs * 30) / 1000,
          fps: 30,
          width: dimensions[0],
          height: dimensions[1],
        },
        inputProps: { fixture },
        container: "mp4",
        videoCodec: "h264",
        muted: true,
        videoBitrate: 8_000_000,
        hardwareAcceleration: "no-preference",
        keyframeIntervalInSeconds: 2,
        pageResponsiveness: "medium",
        licenseKey: "free-license",
        isProduction: false,
        onProgress: (progress) => {
          encodedFrames = progress.encodedFrames;
        },
      });
      const blob = await rendered.getBlob();
      const elapsedMs = performance.now() - started;
      const output = await window.browserMediaBenchmark.verifyOutput(
        fixture.id,
        blob,
      );
      output.reportedEncodedFrames = encodedFrames;
      return {
        status: "done",
        elapsedMs,
        phases: {
          video: elapsedMs,
          nativeAssetPreparation: fixture.nativeAssetPreparationMs ?? null,
        },
        output,
      };
    } catch (error) {
      return {
        status: "failed",
        elapsedMs: performance.now() - started,
        reason: error instanceof Error ? error.message : "EXPERIMENT_FAILED",
      };
    }
  },
});
