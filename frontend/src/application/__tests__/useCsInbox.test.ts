import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useCsConversations } from "../hooks/useCsInbox";

function wrapper(client: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client }, children);
}

describe("useCsConversations", () => {
  // AppLayout asks for the awaiting-reply count on every page, including for
  // a Viewer, who gets 403 from the endpoint. A disabled query must not send
  // that request at all.
  it("does not fetch when disabled", () => {
    const client = new QueryClient();

    const { result } = renderHook(
      () => useCsConversations({ awaitingReply: true }, { enabled: false }),
      { wrapper: wrapper(client) },
    );

    // In TanStack Query v5 a disabled query sits at status "pending" with
    // fetchStatus "idle" — it is the fetchStatus that says no request went out.
    expect(result.current.fetchStatus).toBe("idle");
  });
});
