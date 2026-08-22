import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SystemHealthCard } from "./SystemHealthCard";

describe("SystemHealthCard", () => {
  it("reports each dependency the API says is up", () => {
    render(
      <SystemHealthCard
        health={{
          status: "healthy",
          dependencies: { database: "up", redis: "up" },
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("API Server")).toBeInTheDocument();
    expect(screen.getAllByText("Connected")).toHaveLength(2);
    expect(screen.getByText("Healthy")).toBeInTheDocument();
  });

  it("names the failing dependency instead of showing everything green", () => {
    render(
      <SystemHealthCard
        health={{
          status: "degraded",
          dependencies: { database: "down", redis: "up" },
        }}
        isLoading={false}
      />,
    );

    // The API answered, so only the database row reads as unreachable; marking
    // the API row the same way would hide which dependency to investigate.
    expect(screen.getByText("Degraded")).toBeInTheDocument();
    expect(screen.getByText("Unreachable")).toBeInTheDocument();
    expect(screen.getByText("Database")).toBeInTheDocument();
  });

  it("distinguishes a disabled dependency from a broken one", () => {
    // Redis is optional: the API falls back to in-memory sessions. Showing this
    // as "down" would send an operator chasing a failure that does not exist.
    render(
      <SystemHealthCard
        health={{
          status: "healthy",
          dependencies: { database: "up", redis: "disabled" },
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Not configured")).toBeInTheDocument();
    expect(screen.queryByText("Unreachable")).not.toBeInTheDocument();
  });

  it("falls back to the skeleton instead of crashing on a malformed payload", () => {
    // Reading health.dependencies.database on a payload without dependencies is
    // what crashed the dashboard, so the card must not assume the shape.
    render(
      <SystemHealthCard
        health={{ status: "healthy" } as never}
        isLoading={false}
      />,
    );

    expect(screen.getByText("System Health")).toBeInTheDocument();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
  });

  it("reports the API as unreachable when the request never landed", () => {
    render(
      <SystemHealthCard
        health={{
          status: "unreachable",
          dependencies: { database: "unknown", redis: "unknown" },
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Unreachable")).toBeInTheDocument();
    expect(screen.getAllByText("Unknown")).toHaveLength(2);
  });
});
