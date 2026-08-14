import { describe, it, expect } from "vitest";
import { AxiosError } from "axios";
import {
  ApiError,
  ValidationError,
  UnauthorizedError,
  NotFoundError,
  mapApiError,
} from "../http/errorMapper";

describe("Error Mapper", () => {
  it("should map 401 to UnauthorizedError", () => {
    const axiosError = {
      response: { status: 401, data: {} },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(UnauthorizedError);
    expect(result.statusCode).toBe(401);
    expect(result.code).toBe("UNAUTHORIZED");
  });

  it("should map 404 to NotFoundError", () => {
    const axiosError = {
      response: { status: 404, data: { resource: "User" } },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(NotFoundError);
    expect(result.statusCode).toBe(404);
  });

  it("should map 400 with details to ValidationError", () => {
    const axiosError = {
      response: {
        status: 400,
        data: { code: "VALIDATION_ERROR", details: { email: "Invalid email" } },
      },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(ValidationError);
    expect(result.statusCode).toBe(400);
    expect((result as ValidationError).fields).toEqual({
      email: "Invalid email",
    });
  });

  it("should map unknown errors to generic ApiError", () => {
    const axiosError = {
      response: { status: 500, data: { code: "SERVER_ERROR" } },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(ApiError);
    expect(result.statusCode).toBe(500);
    expect(result.code).toBe("SERVER_ERROR");
  });
});
