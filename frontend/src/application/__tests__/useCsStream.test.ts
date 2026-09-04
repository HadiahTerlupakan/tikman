import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useCsStream } from "../hooks/useCsStream";

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  // The spec's readyState values, which the hook reads to tell "still
  // retrying" from "given up".
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;

  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  closed = false;
  readyState = FakeEventSource.OPEN;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(public url: string) {
    FakeEventSource.instances.push(this);
  }

  /** The browser refusing a response it cannot use — a 502 or a 401 — which
   * closes the connection for good rather than retrying it. */
  fail() {
    this.readyState = FakeEventSource.CLOSED;
    this.onerror?.();
  }

  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    (this.listeners[type] ||= []).push(fn);
  }

  emit(type: string, data: string) {
    for (const fn of this.listeners[type] ?? []) {
      fn(new MessageEvent(type, { data }));
    }
  }

  close() {
    this.closed = true;
  }
}

function wrapper(client: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client }, children);
}

describe("useCsStream", () => {
  beforeEach(() => {
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // The stream carries no truth of its own: an event is a nudge to refetch.
  // Trusting its payload instead would leave the inbox wrong for any message
  // that arrived while the connection was down.
  it("refetches the inbox when a message event arrives", () => {
    const client = new QueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");

    renderHook(() => useCsStream(), { wrapper: wrapper(client) });

    const source = FakeEventSource.instances[0];
    expect(source).toBeDefined();

    source.emit(
      "cs",
      JSON.stringify({ type: "message", conversation_id: "abc" }),
    );

    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["cs", "conversations"] }),
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["cs", "messages", "abc"] }),
    );
  });

  it("marks the thread a customer is writing in, and clears it when they stop", () => {
    const client = new QueryClient();
    const { result } = renderHook(() => useCsStream(), {
      wrapper: wrapper(client),
    });
    const source = FakeEventSource.instances[0];

    act(() =>
      source.emit(
        "cs",
        JSON.stringify({
          type: "typing",
          conversation_id: "abc",
          typing: true,
        }),
      ),
    );
    expect(result.current.typing.abc).toBe(true);

    act(() =>
      source.emit(
        "cs",
        JSON.stringify({ type: "typing", conversation_id: "abc" }),
      ),
    );
    expect(result.current.typing.abc).toBeUndefined();
  });

  // A customer holding down a key sends composing every few seconds. Treating
  // those as the nudge every other event is would reload the inbox list and
  // the whole open thread while they are still writing the first sentence.
  it("refetches nothing while a customer types", () => {
    const client = new QueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderHook(() => useCsStream(), { wrapper: wrapper(client) });

    act(() =>
      FakeEventSource.instances[0].emit(
        "cs",
        JSON.stringify({
          type: "typing",
          conversation_id: "abc",
          typing: true,
        }),
      ),
    );

    expect(invalidate).not.toHaveBeenCalled();
  });

  // A phone that loses signal mid-word never sends its paused, and the line
  // would otherwise stay up for the rest of the shift.
  it("takes the line down on its own when the paused never arrives", async () => {
    vi.useFakeTimers();
    try {
      const client = new QueryClient();
      const { result } = renderHook(() => useCsStream(), {
        wrapper: wrapper(client),
      });

      act(() =>
        FakeEventSource.instances[0].emit(
          "cs",
          JSON.stringify({
            type: "typing",
            conversation_id: "abc",
            typing: true,
          }),
        ),
      );
      expect(result.current.typing.abc).toBe(true);

      act(() => vi.advanceTimersByTime(11_000));
      expect(result.current.typing.abc).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });

  // A reconnect that is answered with a 502 while the API restarts, or a 401
  // once the session expires, closes EventSource permanently — no error the CS
  // can see, just an inbox that quietly stops updating. One deploy used to be
  // enough to do that to every open tab.
  it("opens a fresh stream when the browser gives up on one", () => {
    vi.useFakeTimers();
    try {
      const client = new QueryClient();
      renderHook(() => useCsStream(), { wrapper: wrapper(client) });
      expect(FakeEventSource.instances).toHaveLength(1);

      act(() => FakeEventSource.instances[0].fail());
      act(() => vi.advanceTimersByTime(3_000));

      expect(FakeEventSource.instances).toHaveLength(2);
      expect(FakeEventSource.instances[1].url).toBe(
        FakeEventSource.instances[0].url,
      );
    } finally {
      vi.useRealTimers();
    }
  });

  // EventSource retries a dropped connection by itself. Opening a second one
  // alongside it would leave two streams feeding the same inbox.
  it("leaves a stream that is still retrying alone", () => {
    vi.useFakeTimers();
    try {
      const client = new QueryClient();
      renderHook(() => useCsStream(), { wrapper: wrapper(client) });

      act(() => {
        FakeEventSource.instances[0].readyState = FakeEventSource.CONNECTING;
        FakeEventSource.instances[0].onerror?.();
      });
      act(() => vi.advanceTimersByTime(30_000));

      expect(FakeEventSource.instances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });

  // A page that has navigated away must not be reconnected behind its own
  // back: the reopen is scheduled on a timer that outlives the failure.
  it("does not reopen a stream after the page goes away", () => {
    vi.useFakeTimers();
    try {
      const client = new QueryClient();
      const { unmount } = renderHook(() => useCsStream(), {
        wrapper: wrapper(client),
      });

      unmount();
      act(() => FakeEventSource.instances[0].fail());
      act(() => vi.advanceTimersByTime(30_000));

      expect(FakeEventSource.instances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("closes the connection when the page goes away", () => {
    const client = new QueryClient();
    const { unmount } = renderHook(() => useCsStream(), {
      wrapper: wrapper(client),
    });

    unmount();

    expect(FakeEventSource.instances[0].closed).toBe(true);
  });

  // AppLayout runs this hook on every page for every role, so a Viewer — who
  // gets 403 from /api/v1/cs/stream — must never open the connection at all.
  // EventSource reconnects on its own after an error, so "it fails harmlessly"
  // is not true: it fails in a loop.
  it("opens no connection when disabled", () => {
    const client = new QueryClient();

    renderHook(() => useCsStream(false), { wrapper: wrapper(client) });

    expect(FakeEventSource.instances).toHaveLength(0);
  });

  // Presence puts the agent into the CS round-robin, so a connection held from
  // the OLT map must not claim it — only the inbox route asks.
  it("claims presence only when asked to", () => {
    const client = new QueryClient();

    const { rerender } = renderHook(
      ({ presence }: { presence: boolean }) => useCsStream(true, presence),
      { wrapper: wrapper(client), initialProps: { presence: false } },
    );
    expect(FakeEventSource.instances[0].url).not.toContain("presence");

    // Navigating into the inbox has to reopen the connection: the claim is in
    // the URL, so keeping the old one would leave the agent out of rotation.
    rerender({ presence: true });
    expect(FakeEventSource.instances[0].closed).toBe(true);
    expect(FakeEventSource.instances[1].url).toContain("presence=1");
  });
});
