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

  it("should use the server error text as the message", () => {
    const axiosError = {
      response: {
        status: 500,
        data: {
          code: "SCAN_FAILED",
          error: "SNMP community not configured for this OLT",
        },
      },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result.message).toBe("SNMP community not configured for this OLT");
    expect(result.code).toBe("SCAN_FAILED");
  });

  it("should use the server error text on 404 instead of the resource name", () => {
    const axiosError = {
      response: {
        status: 404,
        data: { code: "NOT_FOUND", error: "OLT not found" },
      },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(NotFoundError);
    expect(result.message).toBe("OLT not found");
  });

  it("should keep the server error text on a 400 that carries details", () => {
    const axiosError = {
      response: {
        status: 400,
        data: {
          code: "SITE_HAS_TUNNEL",
          error: "Site still has a VPN tunnel",
          details:
            "site still has a VPN tunnel: remove the site's tunnel first",
        },
      },
    } as AxiosError;

    const result = mapApiError(axiosError);

    expect(result).toBeInstanceOf(ValidationError);
    expect(result.message).toBe("Site still has a VPN tunnel");
  });

  it("should fall back to the code when the server sends no error text", () => {
    const axiosError = {
      response: { status: 500, data: { code: "SCAN_FAILED" } },
    } as AxiosError;

    expect(mapApiError(axiosError).message).toBe("SCAN_FAILED");
  });
});
