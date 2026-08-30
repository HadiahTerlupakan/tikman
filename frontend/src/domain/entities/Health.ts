export type DependencyStatus = "up" | "down" | "disabled" | "unknown";

export type HealthStatus = "healthy" | "degraded" | "unreachable";

export interface Health {
  status: HealthStatus;
  dependencies: {
    database: DependencyStatus;
    redis: DependencyStatus;
    /** Whether the polling worker finished a cycle recently. */
    worker: DependencyStatus;
  };
  /** When the worker last finished a cycle. Absent until it has run once. */
  workerLastBeat?: string;
}
