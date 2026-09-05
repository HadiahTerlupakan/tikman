import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { CsTeamPanel } from "../CsTeamPanel";
import { UserRole, type User } from "@/domain/entities";

const user = (id: string, username: string, role: UserRole): User =>
  ({ id, username, role }) as User;

const team: User[] = [
  user("u-rina", "Rina", UserRole.CS),
  user("u-budi", "Budi", UserRole.TECHNICIAN),
  user("u-admin", "Admin", UserRole.ADMIN),
  user("u-vera", "Vera", UserRole.VIEWER),
];

function names(): string[] {
  return screen
    .getAllByRole("listitem")
    .map((li) => li.getAttribute("aria-label") ?? "");
}

describe("CsTeamPanel", () => {
  // A Viewer cannot open the inbox at all — the API turns that role away — so
  // listing them would offer a colleague who can never take the thread.
  it("lists only the roles that can open the inbox", () => {
    render(<CsTeamPanel users={team} online={[]} currentUserId="u-rina" />);

    expect(screen.getByText("Rina")).toBeInTheDocument();
    expect(screen.getByText("Budi")).toBeInTheDocument();
    expect(screen.getByText("Admin")).toBeInTheDocument();
    expect(screen.queryByText("Vera")).toBeNull();
  });

  // Sorted by name rather than by status: presence changes on its own every
  // time somebody's browser sleeps, and a list ordered by it would reshuffle
  // under the reader's cursor.
  it("keeps one order whoever is online", () => {
    const { rerender } = render(
      <CsTeamPanel users={team} online={[]} currentUserId="u-rina" />,
    );
    const before = names();

    rerender(
      <CsTeamPanel users={team} online={["u-rina"]} currentUserId="u-rina" />,
    );

    expect(names().map((n) => n.split(" —")[0])).toEqual(
      before.map((n) => n.split(" —")[0]),
    );
    expect(names().map((n) => n.split(" —")[0])).toEqual([
      "Admin",
      "Budi",
      "Rina",
    ]);
  });

  // The dot is a colour, and a colour alone says nothing to a screen reader or
  // to anyone who cannot separate the two greens.
  it("says in words who is at the inbox and who is not", () => {
    render(
      <CsTeamPanel users={team} online={["u-rina"]} currentUserId="u-budi" />,
    );

    expect(screen.getByLabelText(/Rina — sedang di inbox/)).toBeInTheDocument();
    expect(
      screen.getByLabelText(/Budi — sedang tidak di inbox/),
    ).toBeInTheDocument();
  });

  // The legend used to spell out what a green dot meant, in a sentence that
  // took the whole header. A count says the same thing and answers the
  // question the reader actually has: how many of us are here.
  it("counts how many of the team are at the inbox", () => {
    render(
      <CsTeamPanel
        users={team}
        online={["u-rina", "u-admin"]}
        currentUserId="u-rina"
      />,
    );

    expect(screen.getByText(/2 dari 3 di inbox/)).toBeInTheDocument();
  });

  // The Viewer is not in the list, so it must not be in the total either — a
  // count that includes people the panel refuses to show can never reach it.
  it("counts only the roles it lists", () => {
    render(
      <CsTeamPanel users={team} online={["u-vera"]} currentUserId="u-rina" />,
    );

    expect(screen.getByText(/0 dari 3 di inbox/)).toBeInTheDocument();
  });

  // Seeing your own row lit is how a CS knows their own presence registered —
  // the thing the round-robin uses to hand them work.
  it("marks the reader's own row", () => {
    render(
      <CsTeamPanel users={team} online={["u-budi"]} currentUserId="u-budi" />,
    );

    expect(screen.getByText("(Anda)")).toBeInTheDocument();
    expect(screen.getByLabelText(/Budi — sedang di inbox/)).toBeInTheDocument();
  });
});
