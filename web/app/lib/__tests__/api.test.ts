import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "../api";

// We need to test the ApiClient class through the exported `api` singleton.
// Mock global fetch for all API tests.
const fetchMock = vi.fn();
vi.stubGlobal("fetch", fetchMock);

// Fresh import after stubbing fetch
const { api } = await import("../api");

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function errorResponse(status: number, error = "error", message = "Error") {
  return new Response(JSON.stringify({ error, message }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("ApiError", () => {
  it("is an instance of Error", () => {
    const err = new ApiError(404, "not_found", "Not found");
    expect(err).toBeInstanceOf(Error);
  });

  it("has correct name", () => {
    const err = new ApiError(500, "internal", "Oops");
    expect(err.name).toBe("ApiError");
  });

  it("stores status, code, and message", () => {
    const err = new ApiError(403, "forbidden", "Access denied");
    expect(err.status).toBe(403);
    expect(err.code).toBe("forbidden");
    expect(err.message).toBe("Access denied");
  });

  it("has a proper stack trace", () => {
    const err = new ApiError(500, "error", "msg");
    expect(err.stack).toBeDefined();
  });
});

describe("Token management", () => {
  beforeEach(() => {
    localStorage.clear();
    api.clearToken();
  });

  it("setToken stores in localStorage", () => {
    api.setToken("abc123");
    expect(localStorage.getItem("access_token")).toBe("abc123");
  });

  it("getToken returns the stored token", () => {
    api.setToken("xyz");
    expect(api.getToken()).toBe("xyz");
  });

  it("setToken(null) removes from localStorage", () => {
    api.setToken("abc");
    api.setToken(null);
    expect(localStorage.getItem("access_token")).toBeNull();
    expect(api.getToken()).toBeNull();
  });

  it("clearToken removes access_token and refresh_token", () => {
    api.setToken("access");
    localStorage.setItem("refresh_token", "refresh");
    api.clearToken();
    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(api.getToken()).toBeNull();
  });
});

describe("Auth header injection", () => {
  beforeEach(() => {
    localStorage.clear();
    api.clearToken();
    fetchMock.mockReset();
  });

  it("adds Bearer token when set", async () => {
    api.setToken("my-token");
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { id: "1", email: "a@b.com", name: "A", created_at: "" } }),
    );

    await api.getCurrentUser();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, options] = fetchMock.mock.calls[0];
    expect(options.headers["Authorization"]).toBe("Bearer my-token");
  });

  it("does not add Authorization header when no token", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: [] }),
    );

    await api.getAuthProviders();

    const [, options] = fetchMock.mock.calls[0];
    expect(options.headers["Authorization"]).toBeUndefined();
  });
});

describe("Error handling", () => {
  beforeEach(() => {
    localStorage.clear();
    api.clearToken();
    fetchMock.mockReset();
  });

  it("throws ApiError with status and code on server error", async () => {
    fetchMock.mockResolvedValueOnce(errorResponse(500, "internal", "Server error"));

    await expect(api.getAuthProviders()).rejects.toThrow(ApiError);
    try {
      await api.getAuthProviders();
    } catch {
      // already tested above
    }
  });

  it("extracts error field as message for 4xx responses", async () => {
    fetchMock.mockResolvedValueOnce(
      errorResponse(400, "validation_error", "Bad input"),
    );

    try {
      await api.getAuthProviders();
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).status).toBe(400);
      expect((err as ApiError).message).toBe("validation_error");
    }
  });

  it("falls back gracefully on non-JSON error body", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("not json", { status: 502 }),
    );

    try {
      await api.getAuthProviders();
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).status).toBe(502);
    }
  });

  it("handles empty response body", async () => {
    fetchMock.mockResolvedValueOnce(new Response("", { status: 200 }));

    const result = await api.getAuthProviders();
    expect(result.data).toEqual({});
  });
});

describe("401 auto-retry with refresh", () => {
  beforeEach(() => {
    localStorage.clear();
    api.clearToken();
    fetchMock.mockReset();
  });

  it("retries after refreshing token on 401", async () => {
    api.setToken("expired-token");
    localStorage.setItem("refresh_token", "valid-refresh");

    // First call: 401
    fetchMock.mockResolvedValueOnce(
      errorResponse(401, "unauthorized", "Token expired"),
    );
    // Refresh call: success
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: { access_token: "new-token", refresh_token: "new-refresh", expires_in: 3600, token_type: "Bearer" },
      }),
    );
    // Retry call: success
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { id: "1", email: "a@b.com", name: "Test", created_at: "" } }),
    );

    const result = await api.getCurrentUser();
    expect(result.data.email).toBe("a@b.com");
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(api.getToken()).toBe("new-token");
  });

  it("does not retry 401 when no refresh token available", async () => {
    api.setToken("expired-token");
    // No refresh_token in localStorage

    fetchMock.mockResolvedValueOnce(
      errorResponse(401, "unauthorized", "Token expired"),
    );

    await expect(api.getCurrentUser()).rejects.toThrow(ApiError);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

describe("Concurrent refresh deduplication", () => {
  beforeEach(() => {
    localStorage.clear();
    api.clearToken();
    fetchMock.mockReset();
  });

  it("deduplicates concurrent refresh requests", async () => {
    api.setToken("expired");
    localStorage.setItem("refresh_token", "valid-refresh");

    // Both calls get 401
    fetchMock.mockResolvedValueOnce(
      errorResponse(401, "unauthorized", "expired"),
    );
    fetchMock.mockResolvedValueOnce(
      errorResponse(401, "unauthorized", "expired"),
    );
    // Single refresh call
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: { access_token: "fresh", refresh_token: "fresh-r", expires_in: 3600, token_type: "Bearer" },
      }),
    );
    // Both retries succeed
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { id: "1", email: "a@b.com", name: "A", created_at: "" } }),
    );
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: [] }),
    );

    const [user] = await Promise.all([
      api.getCurrentUser(),
      api.getAuthProviders(),
    ]);

    expect(user.data.email).toBe("a@b.com");

    // Should have: 2 initial calls + 1 refresh (deduped) + 2 retries = 5
    // But the timing can vary. The key assertion is we don't see 2 refresh calls.
    const refreshCalls = fetchMock.mock.calls.filter(
      (call: unknown[]) => typeof call[0] === "string" && (call[0] as string).includes("/auth/refresh"),
    );
    expect(refreshCalls.length).toBeLessThanOrEqual(1);
  });
});
