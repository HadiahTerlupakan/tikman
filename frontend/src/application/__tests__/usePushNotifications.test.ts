import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { act, renderHook } from "@testing-library/react";

const configured = vi.hoisted(() => ({ value: false }));

vi.mock("@/shared/config/firebase", () => ({
  get isFirebaseConfigured() {
    return configured.value;
  },
}));

vi.mock("@/infrastructure/firebase/messaging", () => ({
  requestPushPermission: vi.fn(),
  registerForPush: vi.fn(),
}));

const { requestPushPermission, registerForPush } = await import(
  "@/infrastructure/firebase/messaging"
);
const mockRequestPermission = vi.mocked(requestPushPermission);
const mockRegisterForPush = vi.mocked(registerForPush);

const { usePushNotifications } = await import(
  "@/application/hooks/usePushNotifications"
);

describe("usePushNotifications", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it("registers with FCM once the browser grants permission", async () => {
    configured.value = true;
    mockRequestPermission.mockResolvedValue("granted");
    mockRegisterForPush.mockResolvedValue(undefined);

    const { result } = renderHook(() => usePushNotifications());
    await act(() => result.current.enable());

    expect(result.current.permission).toBe("granted");
    expect(mockRegisterForPush).toHaveBeenCalled();
  });

  // Asking again after a block would spend nothing and register nothing; the
  // browser has already decided.
  it("does not register when permission is refused", async () => {
    configured.value = true;
    mockRequestPermission.mockResolvedValue("denied");

    const { result } = renderHook(() => usePushNotifications());
    await act(() => result.current.enable());

    expect(result.current.permission).toBe("denied");
    expect(mockRegisterForPush).not.toHaveBeenCalled();
  });
});
