import assert from "node:assert/strict";
import test from "node:test";

const completionSound = await import("../src/lib/completionSound.ts");

test("completion sound only plays when the final job settles", () => {
  const config = completionSound.normalizeCompletionSoundConfig({ enabled: true });
  assert.equal(completionSound.shouldPlayCompletionSound({
    config,
    completedNow: 1,
    totalNow: 3,
  }), false);
  assert.equal(completionSound.shouldPlayCompletionSound({
    config,
    completedNow: 3,
    totalNow: 3,
  }), true);
});

test("disabled completion sound never plays unless forced preview is used", async () => {
  let playCalls = 0;
  const played = await completionSound.playCompletionSound(
    completionSound.normalizeCompletionSoundConfig({ enabled: false }),
    {
      createAudio: () => ({
        currentTime: 0,
        play: () => {
          playCalls += 1;
        },
      }),
    },
  );
  assert.equal(played, false);
  assert.equal(playCalls, 0);
});

test("completion sound preview can force playback", async () => {
  let playCalls = 0;
  const played = await completionSound.playCompletionSound(
    completionSound.normalizeCompletionSoundConfig({ enabled: false }),
    {
      force: true,
      createAudio: () => ({
        currentTime: 0,
        play: () => {
          playCalls += 1;
        },
      }),
    },
  );
  assert.equal(played, true);
  assert.equal(playCalls, 1);
});

test("blocked audio reports failed playback without losing the visible failure alert", async () => {
  for (const play of [() => Promise.reject(new Error("NotAllowedError")), () => { throw new Error("audio unavailable"); }]) {
    assert.equal(await completionSound.playCompletionSound(
      completionSound.normalizeCompletionSoundConfig({ enabled: true }),
      { createAudio: () => ({ currentTime: 0, play }) },
    ), false);
  }
});

test("importCompletionSoundFile rejects oversized files", async () => {
  const bytes = new Uint8Array(completionSound.MAX_COMPLETION_SOUND_BYTES + 1);
  const file = new File([bytes], "too-large.wav", { type: "audio/wav" });
  await assert.rejects(
    completionSound.importCompletionSoundFile(file),
    /不能超过/,
  );
});

test("importCompletionSoundFile accepts supported audio and returns a data URL", async () => {
  const file = new File([new Uint8Array([1, 2, 3, 4])], "ding.wav", { type: "audio/wav" });
  const imported = await completionSound.importCompletionSoundFile(file);
  assert.equal(imported.name, "ding.wav");
  assert.match(imported.dataURL, /^data:audio\/wav;base64,/);
});
