import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useCsStream } from "../hooks/useCsStream";

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  closed = false;

  constructor(public url: string) {
    FakeEventSource.instances.push(this);
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

  it("closes the connection when the page goes away", () => {
    const client = new QueryClient();
    const { unmount } = renderHook(() => useCsStream(), {
      wrapper: wrapper(client),
    });

    unmount();

    expect(FakeEventSource.instances[0].closed).toBe(true);
  });
});
