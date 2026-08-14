import type { AxiosError } from 'axios';

export class ApiError extends Error {
  constructor(
    public statusCode: number,
    public code: string,
    public details?: Record<string, any>
  ) {
    super();
    this.name = 'ApiError';
    this.message = code;
  }
}

export class ValidationError extends ApiError {
  constructor(public fields: Record<string, string>) {
    super(400, 'VALIDATION_ERROR', fields);
    this.name = 'ValidationError';
  }
}

export class UnauthorizedError extends ApiError {
  constructor() {
    super(401, 'UNAUTHORIZED');
    this.name = 'UnauthorizedError';
    this.message = 'Session expired or invalid';
  }
}

export class NotFoundError extends ApiError {
  constructor(resource: string) {
    super(404, 'NOT_FOUND', { resource });
    this.name = 'NotFoundError';
    this.message = `${resource} not found`;
  }
}

export function mapApiError(error: AxiosError): ApiError {
  const response = error.response?.data as any;

  if (error.response?.status === 401) {
    return new UnauthorizedError();
  }

  if (error.response?.status === 404) {
    return new NotFoundError(response?.resource || 'Resource');
  }

  if (error.response?.status === 400 && response?.details) {
    return new ValidationError(response.details);
  }

  return new ApiError(
    error.response?.status || 500,
    response?.code || 'UNKNOWN_ERROR',
    response?.details
  );
}
