import type { AxiosError } from "axios";

interface ApiErrorResponse {
  code?: string;
  error?: string;
  resource?: string;
  details?: Record<string, string>;
}

export class ApiError extends Error {
  constructor(
    public statusCode: number,
    public code: string,
    public details?: Record<string, unknown>,
    message?: string,
  ) {
    super();
    this.name = "ApiError";
    this.message = message || code;
  }
}

export class ValidationError extends ApiError {
  constructor(
    public fields: Record<string, string>,
    message?: string,
  ) {
    super(400, "VALIDATION_ERROR", fields, message);
    this.name = "ValidationError";
  }
}

export class UnauthorizedError extends ApiError {
  constructor() {
    super(401, "UNAUTHORIZED");
    this.name = "UnauthorizedError";
    this.message = "Session expired or invalid";
  }
}

export class NotFoundError extends ApiError {
  constructor(resource: string, message?: string) {
    super(404, "NOT_FOUND", { resource }, message || `${resource} not found`);
    this.name = "NotFoundError";
  }
}

export function mapApiError(error: AxiosError): ApiError {
  const response = error.response?.data as ApiErrorResponse | undefined;

  if (error.response?.status === 401) {
    return new UnauthorizedError();
  }

  if (error.response?.status === 404) {
    return new NotFoundError(response?.resource || "Resource", response?.error);
  }

  if (error.response?.status === 400 && response?.details) {
    return new ValidationError(response.details, response.error);
  }

  return new ApiError(
    error.response?.status || 500,
    response?.code || "UNKNOWN_ERROR",
    response?.details,
    response?.error,
  );
}
