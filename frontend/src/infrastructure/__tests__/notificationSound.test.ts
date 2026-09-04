import { describe, expect, it, vi, afterEach, beforeEach } from "vitest";

// The module keeps one AudioContext for the life of the page — browsers cap
// how many may exist — so each test needs its own module instance, or the
// context the previous test stubbed is the one still in use.
async function loadChime() {
  vi.resetModules();
  return (await import("../notificationSound")).playNotificationChime;
}

describe("playNotificationChime", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // The chime accompanies a notification that has already been delivered. A
  // browser with no Web Audio, or one refusing to start a context, must leave
  // the message delivered rather than turn it into a rejected promise nobody
  // is catching. jsdom has no AudioContext, so this is the real path here.
  it("resolves quietly where the browser cannot play audio", async () => {
    const playNotificationChime = await loadChime();
    await expect(playNotificationChime()).resolves.toBeUndefined();
  });

  it("resolves quietly when starting the context is refused", async () => {
    class RefusingContext {
      state = "suspended";
      currentTime = 0;
      destination = {};
      resume() {
        return Promise.reject(new Error("not allowed"));
      }
      createOscillator() {
        throw new Error("should not reach this");
      }
      createGain() {
        throw new Error("should not reach this");
      }
    }
    vi.stubGlobal("AudioContext", RefusingContext);

    const playNotificationChime = await loadChime();
    await expect(playNotificationChime()).resolves.toBeUndefined();
  });

  it("plays both notes of the chime when audio is available", async () => {
    const started: number[] = [];
    const oscillator = () => ({
      type: "",
      frequency: { setValueAtTime: vi.fn() },
      connect: vi.fn(),
      start: (at: number) => started.push(at),
      stop: vi.fn(),
    });
    const gain = () => ({
      gain: {
        setValueAtTime: vi.fn(),
        exponentialRampToValueAtTime: vi.fn(),
      },
      connect: vi.fn(),
    });

    class RunningContext {
      state = "running";
      currentTime = 0;
      destination = {};
      resume = vi.fn();
      createOscillator = oscillator;
      createGain = gain;
    }
    vi.stubGlobal("AudioContext", RunningContext);

    const playNotificationChime = await loadChime();
    await playNotificationChime();

    // Two notes, the second following the first rather than sounding together
    // — a chord would read as an alarm, not an arrival.
    expect(started).toHaveLength(2);
    expect(started[1]).toBeGreaterThan(started[0]);
  });
});
