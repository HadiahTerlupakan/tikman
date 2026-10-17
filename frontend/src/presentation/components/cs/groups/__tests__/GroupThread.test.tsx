import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { GroupThread } from "../GroupThread";
import { waGroupMessage } from "./fixtures";

describe("GroupThread", () => {
  // Newest first, as the API sends them.
  const messages = [
    waGroupMessage({
      id: "m4",
      direction: "out",
      body: "siap",
      senderName: "",
    }),
    waGroupMessage({
      id: "m3",
      senderJid: "628222@s.whatsapp.net",
      senderName: "Ani",
      body: "saya cek",
    }),
    waGroupMessage({ id: "m2", body: "di Selawangi" }),
    waGroupMessage({ id: "m1", body: "ODP 3 mati" }),
  ];

  it("names each sender once per run of their messages", () => {
    render(<GroupThread messages={messages} onRetry={vi.fn()} />);

    expect(screen.getAllByText("Budi")).toHaveLength(1);
    expect(screen.getAllByText("Ani")).toHaveLength(1);
  });

  it("puts no sender name above our own messages", () => {
    render(<GroupThread messages={messages} onRetry={vi.fn()} />);

    expect(screen.queryByText("Anda")).not.toBeInTheDocument();
    expect(screen.getByText("siap")).toBeInTheDocument();
  });

  it("draws the thread oldest first", () => {
    render(<GroupThread messages={messages} onRetry={vi.fn()} />);

    const bodies = ["ODP 3 mati", "di Selawangi", "saya cek", "siap"].map(
      (text) => screen.getByText(text),
    );
    for (let i = 1; i < bodies.length; i++) {
      expect(
        bodies[i - 1].compareDocumentPosition(bodies[i]) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
  });

  it("loads a group photo from the group media endpoint, not the inbox's", () => {
    render(
      <GroupThread
        messages={[
          waGroupMessage({ kind: "image", body: "", mediaFilename: "odp.jpg" }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByAltText("odp.jpg").getAttribute("src")).toContain(
      "/api/v1/cs/group-media/m1",
    );
  });

  it("names the member a reply quotes", () => {
    render(
      <GroupThread
        messages={[
          waGroupMessage({
            id: "m2",
            senderName: "Ani",
            senderJid: "628222@s.whatsapp.net",
            body: "sudah",
            replyTo: {
              id: "m1",
              direction: "in",
              kind: "text",
              body: "ODP 3 mati",
              senderName: "Budi",
            },
          }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByText("Budi")).toBeInTheDocument();
    expect(screen.getByText("ODP 3 mati")).toBeInTheDocument();
  });
});
