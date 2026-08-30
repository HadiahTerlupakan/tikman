import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SystemHealthCard } from "./SystemHealthCard";

describe("SystemHealthCard", () => {
  it("reports each dependency the API says is up", () => {
    render(
      <SystemHealthCard
        health={{
          status: "healthy",
          dependencies: { database: "up", redis: "up", worker: "up" },
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
          dependencies: { database: "down", redis: "up", worker: "up" },
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
          dependencies: { database: "up", redis: "disabled", worker: "up" },
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
          dependencies: {
            database: "unknown",
            redis: "unknown",
            worker: "unknown",
          },
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Unreachable")).toBeInTheDocument();
    expect(screen.getAllByText("Unknown")).toHaveLength(3);
  });

  it("backs a polling worker with the age of its last cycle", () => {
    // Every other row can be green while the worker is dead, so "Polling" on
    // its own is the one claim here the operator cannot check.
    render(
      <SystemHealthCard
        health={{
          status: "healthy",
          dependencies: { database: "up", redis: "up", worker: "up" },
          workerLastBeat: new Date(Date.now() - 20_000).toISOString(),
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Polling Worker")).toBeInTheDocument();
    expect(screen.getByText("Polling")).toBeInTheDocument();
    expect(screen.getByText("cycle 20s ago")).toBeInTheDocument();
  });

  it("says how long ago polling stopped, while the API stays healthy", () => {
    // This is the failure the card exists for: the ONT figures on the page are
    // frozen, and nothing else on it would say so.
    render(
      <SystemHealthCard
        health={{
          status: "healthy",
          dependencies: { database: "up", redis: "up", worker: "down" },
          workerLastBeat: new Date(Date.now() - 7_200_000).toISOString(),
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Stopped")).toBeInTheDocument();
    expect(screen.getByText("last cycle 2h ago")).toBeInTheDocument();
    expect(screen.getByText("Healthy")).toBeInTheDocument();
  });

  it("does not call a fresh install a fault", () => {
    render(
      <SystemHealthCard
        health={{
          status: "healthy",
          dependencies: { database: "up", redis: "up", worker: "unknown" },
        }}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Never run")).toBeInTheDocument();
  });

  it("survives an API that predates the worker row", () => {
    // The API and this page deploy as separate containers. A frontend that
    // rolls out first receives no worker status, and this used to be exactly
    // the shape that blanked the whole dashboard.
    render(
      <SystemHealthCard
        health={
          {
            status: "healthy",
            dependencies: { database: "up", redis: "up" },
          } as never
        }
        isLoading={false}
      />,
    );

    expect(screen.getByText("Polling Worker")).toBeInTheDocument();
    expect(screen.getAllByText("Connected")).toHaveLength(2);
  });

  it("does not claim the worker never ran when it could not be asked", () => {
    // An unreachable API tells us nothing about the worker. "Never run" would
    // report a fresh install where there is a connection problem.
    render(
      <SystemHealthCard
        health={{
          status: "unreachable",
          dependencies: {
            database: "unknown",
            redis: "unknown",
            worker: "unknown",
          },
        }}
        isLoading={false}
      />,
    );

    expect(screen.queryByText("Never run")).not.toBeInTheDocument();
  });
});
