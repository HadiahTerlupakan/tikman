export type DependencyStatus = "up" | "down" | "disabled" | "unknown";

export type HealthStatus = "healthy" | "degraded" | "unreachable";

export interface Health {
  status: HealthStatus;
  dependencies: {
    database: DependencyStatus;
    redis: DependencyStatus;
  };
}
