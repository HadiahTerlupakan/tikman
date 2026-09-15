import { afterEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThreadPane } from "../ThreadPane";
import type { CsConversation, CsMessage } from "@/domain/entities";

const conversation = {
  id: "c1",
  customerPhone: "628111222333",
  customerName: "Budi",
  status: "open" as const,
  lastMessageAt: "2026-09-15T01:00:00Z",
  unreadCount: 0,
  hasAvatar: false,
  assignedUserId: "me",
} as CsConversation;

function message(id: string, direction: "in" | "out"): CsMessage {
  return {
    id,
    conversationId: "c1",
    direction,
    kind: "text",
    body: `pesan ${id}`,
    status: "delivered",
    waTimestamp: "2026-09-15T01:00:00Z",
    createdAt: "2026-09-15T01:00:00Z",
  } as CsMessage;
}

// History arrives newest first, the way the API sends it.
const history = (last: "in" | "out") => [
  message("m3", last),
  message("m2", "out"),
  message("m1", "in"),
];

function pane(messages: CsMessage[], loading = false) {
  return (
    <ThreadPane
      conversation={conversation}
      messages={messages}
      loading={loading}
      currentUserId="me"
      holderNames={{}}
      users={[]}
      quickReplies={[]}
      sending={false}
      attaching={false}
      transferring={false}
      clearing={false}
      canPurge={false}
      onSend={vi.fn().mockResolvedValue(true)}
      onAttach={vi.fn().mockResolvedValue(true)}
      onTakeOver={vi.fn()}
      onTransfer={vi.fn()}
      onReply={vi.fn()}
      onCancelReply={vi.fn()}
      onDeleteMessage={vi.fn()}
      onClearThread={vi.fn()}
      customerTyping={false}
      onTypingChange={vi.fn()}
    />
  );
}

/** jsdom lays nothing out, so the log is given the sizes a browser would
 * report: a 400px window onto a taller history. */
function sized(log: HTMLElement) {
  const size = { content: 1200 };
  Object.defineProperty(log, "clientHeight", {
    configurable: true,
    value: 400,
  });
  Object.defineProperty(log, "scrollHeight", {
    configurable: true,
    get: () => size.content,
  });
  Object.defineProperty(log, "scrollTop", {
    configurable: true,
    writable: true,
    value: 0,
  });
  return size;
}

/** Opens the thread the way the inbox does: a spinner first, history after. */
function openThread(last: "in" | "out" = "in") {
  const view = render(pane([], true));
  const log = screen.getByRole("log");
  const size = sized(log);
  view.rerender(pane(history(last)));
  return { view, log, size };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("the conversation log", () => {
  it.each(["in", "out"] as const)(
    "opens at the newest message when the last one is %s",
    (last) => {
      const { log, size } = openThread(last);

      expect(log.scrollTop).toBe(size.content);
    },
  );

  it("follows a new message while the reader is at the end", () => {
    const { view, log, size } = openThread();

    size.content = 1300;
    view.rerender(pane([message("m4", "in"), ...history("in")]));

    expect(log.scrollTop).toBe(1300);
  });

  // Yanking someone down while they read an old message loses their place.
  it("leaves a reader who scrolled up where they are", () => {
    const { view, log, size } = openThread();

    log.scrollTop = 200;
    fireEvent.scroll(log);
    size.content = 1300;
    view.rerender(pane([message("m4", "in"), ...history("in")]));

    expect(log.scrollTop).toBe(200);
  });

  it("brings the reader down when they send a reply", async () => {
    const { log, size } = openThread();
    log.scrollTop = 200;
    fireEvent.scroll(log);

    await userEvent.type(screen.getByPlaceholderText("Tulis balasan"), "halo");
    await userEvent.click(screen.getByRole("button", { name: /^kirim$/i }));

    expect(log.scrollTop).toBe(size.content);
  });

  // A photo or link preview takes its height only once it has loaded, after
  // the render that placed it — without this the view stops just short.
  it("stays at the end when content grows after it rendered", () => {
    let grew: () => void = () => {};
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          grew = callback;
        }
        observe() {}
        disconnect() {}
      },
    );
    const { log, size } = openThread();

    size.content = 1500;
    grew();

    expect(log.scrollTop).toBe(1500);
  });
});
