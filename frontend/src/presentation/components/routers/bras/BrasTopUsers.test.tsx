import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrasTopUsers } from "./BrasTopUsers";

describe("BrasTopUsers", () => {
  it("lists each user with a copy button and both rates", () => {
    render(
      <BrasTopUsers
        users={[{ name: "pel-0001", downBps: 186_000_000, upBps: 9_000_000 }]}
        queuesFound
      />,
    );
    const row = screen.getByText("pel-0001").closest("tr") as HTMLElement;
    expect(row.querySelector(".ant-typography-copy")).not.toBeNull();
    expect(row).toHaveTextContent("186");
    expect(row).toHaveTextContent("9");
  });

  it("shows and copies a PPPoE queue as the bare username", () => {
    render(
      <BrasTopUsers
        users={[
          { name: "<pppoe-pel-0002>", downBps: 1_000_000, upBps: 0 },
          { name: "hotspot-lobby", downBps: 500_000, upBps: 0 },
        ]}
        queuesFound
      />,
    );
    expect(screen.getByText("pel-0002")).toBeVisible();
    expect(screen.queryByText("<pppoe-pel-0002>")).toBeNull();
    expect(screen.getByText("hotspot-lobby")).toBeVisible();
  });

  it("explains an empty list when the BRAS has no dynamic queues", () => {
    render(<BrasTopUsers users={[]} queuesFound={false} />);
    expect(
      screen.getByText(
        "Tidak ada antrean dinamis: batas kecepatan tidak diberikan oleh RADIUS atau profil.",
      ),
    ).toBeVisible();
  });
});
