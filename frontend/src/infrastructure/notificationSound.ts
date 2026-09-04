// A web notification cannot carry its own sound: the Notification API's `sound`
// option was dropped from the spec and never shipped, so anything the OS
// displays uses the system tone. What a page CAN do is play its own audio while
// a tab is open — which is the case that matters here, a CS sitting with the
// inbox in front of them.
//
// The tone is synthesised rather than shipped as a file: no asset to load
// before the first message, nothing to keep in the repository, and none of the
// copyright trouble that comes with lifting a sound from another platform.

/** Two sine tones a perfect fifth apart, the second following the first — the
 * shape most systems use for "something arrived", and pitched high enough to
 * carry over a room without being shrill. */
const NOTES = [
  { frequency: 880, startAt: 0 }, // A5
  { frequency: 1318.51, startAt: 0.12 }, // E6
];

const NOTE_LENGTH = 0.42;
/** Quiet on purpose. This fires on every incoming message, all day, in a room
 * with other people in it. */
const PEAK_GAIN = 0.12;

type AudioContextConstructor = typeof AudioContext;

let context: AudioContext | undefined;

function audioContext(): AudioContext | undefined {
  const Ctor: AudioContextConstructor | undefined =
    window.AudioContext ??
    (window as unknown as { webkitAudioContext?: AudioContextConstructor })
      .webkitAudioContext;
  if (!Ctor) return undefined;
  // One context for the life of the page: browsers cap how many can exist, and
  // creating one per message exhausts that within a busy afternoon.
  context ??= new Ctor();
  return context;
}

/**
 * Plays the incoming-message chime. Never throws and never rejects: the sound
 * is an accompaniment to a notification that has already been shown, so a
 * browser that blocks audio, or a device with none, must not turn a delivered
 * message into an error.
 *
 * Autoplay policy means a context created before the user has interacted with
 * the page starts suspended. Resuming is attempted, and simply does nothing
 * until the first click — by which point the CS has answered a prompt or
 * clicked into the inbox anyway.
 */
export async function playNotificationChime(): Promise<void> {
  try {
    const ctx = audioContext();
    if (!ctx) return;
    if (ctx.state === "suspended") await ctx.resume();
    if (ctx.state !== "running") return;

    for (const note of NOTES) {
      const oscillator = ctx.createOscillator();
      const gain = ctx.createGain();
      const startAt = ctx.currentTime + note.startAt;

      oscillator.type = "sine";
      oscillator.frequency.setValueAtTime(note.frequency, startAt);

      // A hard start or stop on a sine wave clicks. Ramping in over a few
      // milliseconds and decaying away is what makes it read as a chime rather
      // than a beep.
      gain.gain.setValueAtTime(0.0001, startAt);
      gain.gain.exponentialRampToValueAtTime(PEAK_GAIN, startAt + 0.015);
      gain.gain.exponentialRampToValueAtTime(0.0001, startAt + NOTE_LENGTH);

      oscillator.connect(gain);
      gain.connect(ctx.destination);
      oscillator.start(startAt);
      oscillator.stop(startAt + NOTE_LENGTH);
    }
  } catch {
    // Deliberately swallowed — see the doc comment.
  }
}
