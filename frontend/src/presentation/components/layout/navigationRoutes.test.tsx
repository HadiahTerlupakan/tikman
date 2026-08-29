import { describe, expect, it } from "vitest";
import { UserRole } from "@/domain/entities";
import { buildNavigationRoutes } from "./navigationRoutes";

const paths = (role?: UserRole) =>
  buildNavigationRoutes(role).map((route) => route.path);

describe("buildNavigationRoutes", () => {
  it("lists VPN, so the page is reachable without typing its URL", () => {
    // The VPN entry was once added to a sidebar component nothing rendered,
    // which left the page invisible to every operator.
    expect(paths(UserRole.ADMIN)).toContain("/vpn");
    expect(paths(UserRole.VIEWER)).toContain("/vpn");
  });

  it("shows Users only to an admin", () => {
    expect(paths(UserRole.ADMIN)).toContain("/users");
    expect(paths(UserRole.TECHNICIAN)).not.toContain("/users");
    expect(paths(undefined)).not.toContain("/users");
  });

  it("gives every entry a path and a name", () => {
    for (const route of buildNavigationRoutes(UserRole.ADMIN)) {
      expect(route.path).toMatch(/^\//);
      expect(route.name.length).toBeGreaterThan(0);
    }
  });
});
