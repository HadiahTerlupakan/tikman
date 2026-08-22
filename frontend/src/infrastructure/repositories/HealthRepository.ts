import { apiClient } from "../http/apiClient";
import type { DependencyStatus, Health } from "@/domain/entities";

const UNREACHABLE: Health = {
  status: "unreachable",
  dependencies: { database: "unknown", redis: "unknown" },
};

const DEPENDENCY_STATUSES: DependencyStatus[] = [
  "up",
  "down",
  "disabled",
  "unknown",
];

function toDependencyStatus(value: unknown): DependencyStatus {
  return DEPENDENCY_STATUSES.includes(value as DependencyStatus)
    ? (value as DependencyStatus)
    : "unknown";
}

// A 200 does not guarantee the health payload: any unproxied path falls through
// to the SPA's index.html, so the body has to be validated rather than trusted.
function parse(body: unknown): Health | null {
  if (typeof body !== "object" || body === null) {
    return null;
  }

  const { status, dependencies } = body as Record<string, unknown>;
  if (typeof dependencies !== "object" || dependencies === null) {
    return null;
  }

  const deps = dependencies as Record<string, unknown>;
  return {
    status:
      status === "healthy" || status === "degraded" ? status : "unreachable",
    dependencies: {
      database: toDependencyStatus(deps.database),
      redis: toDependencyStatus(deps.redis),
    },
  };
}

export class HealthRepository {
  // /health answers 503 when a dependency is down, so the failure body is the
  // interesting case rather than an error to propagate.
  async get(): Promise<Health> {
    try {
      const response = await apiClient.get("/health");
      return parse(response.data) ?? UNREACHABLE;
    } catch (error) {
      const body = (error as { response?: { data?: unknown } })?.response?.data;
      return parse(body) ?? UNREACHABLE;
    }
  }
}
