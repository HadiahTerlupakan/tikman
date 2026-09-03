import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

const configured = vi.hoisted(() => ({ value: false }));

vi.mock("@/shared/config/firebase", () => ({
  get isFirebaseConfigured() {
    return configured.value;
  },
}));

vi.mock("@/infrastructure/firebase/messaging", () => ({
  requestPushPermission: vi.fn(),
}));

vi.mock("@/infrastructure/repositories", () => ({
  PushRepository: class {
    subscribe = vi.fn();
    unsubscribe = vi.fn();
  },
}));

const { usePushNotifications } = await import(
  "@/application/hooks/usePushNotifications"
);

describe("usePushNotifications", () => {
  beforeEach(() => {
    vi.stubGlobal("Notification", { permission: "default" });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // Without a Firebase project, asking for permission spends the browser's one
  // prompt and still yields no token — and the button would then claim push was
  // active. "unsupported" is what makes PushOptInButton render nothing.
  it("reports unsupported while Firebase is not configured", () => {
    configured.value = false;

    const { result } = renderHook(() => usePushNotifications());

    expect(result.current.permission).toBe("unsupported");
  });

  it("reports the browser's own permission once Firebase is configured", () => {
    configured.value = true;

    const { result } = renderHook(() => usePushNotifications());

    expect(result.current.permission).toBe("default");
  });
});
