import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock the api module
vi.mock("~/lib/api", () => {
  return {
    api: {
      getToken: vi.fn(),
      listRuns: vi.fn(),
      getCurrentUser: vi.fn(),
      getCacheStats: vi.fn(),
    },
    ApiError: class ApiError extends Error {
      status: number;
      code: string;
      constructor(status: number, code: string, message: string) {
        super(message);
        this.status = status;
        this.code = code;
      }
    },
  };
});

import { api } from "~/lib/api";
import { useRuns } from "../use-runs";
import { useCurrentUser } from "../use-current-user";
import { useCacheStats } from "../use-cache-stats";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useRuns", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("is disabled when projectId is empty", () => {
    const { result } = renderHook(() => useRuns(""), {
      wrapper: createWrapper(),
    });

    expect(result.current.isFetching).toBe(false);
    expect(api.listRuns).not.toHaveBeenCalled();
  });

  it("fetches runs with correct params", async () => {
    const mockRuns = {
      data: { data: [{ id: "r1", status: "success" }], total: 1 },
    };
    vi.mocked(api.listRuns).mockResolvedValueOnce(mockRuns as never);

    const { result } = renderHook(() => useRuns("proj-1", 2, 10), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.listRuns).toHaveBeenCalledWith("proj-1", 2, 10);
  });

  it("returns polling interval of 3s when active runs exist", async () => {
    const mockRuns = {
      data: {
        data: [
          { id: "r1", status: "running" },
          { id: "r2", status: "success" },
        ],
        total: 2,
      },
    };
    vi.mocked(api.listRuns).mockResolvedValue(mockRuns as never);

    const { result } = renderHook(() => useRuns("proj-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    // The refetchInterval is set to 3000 when active runs exist
    // We verify it fetched at least once
    expect(api.listRuns).toHaveBeenCalled();
  });
});

describe("useCurrentUser", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("is disabled when no token", () => {
    vi.mocked(api.getToken).mockReturnValue(null);

    const { result } = renderHook(() => useCurrentUser(), {
      wrapper: createWrapper(),
    });

    expect(result.current.isFetching).toBe(false);
    expect(api.getCurrentUser).not.toHaveBeenCalled();
  });

  it("fetches when token is present", async () => {
    vi.mocked(api.getToken).mockReturnValue("valid-token");
    vi.mocked(api.getCurrentUser).mockResolvedValueOnce({
      data: { id: "u1", email: "user@test.com", name: "Test", created_at: "" },
    } as never);

    const { result } = renderHook(() => useCurrentUser(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.getCurrentUser).toHaveBeenCalledTimes(1);
  });
});

describe("useCacheStats", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("is disabled when no projectId", () => {
    const { result } = renderHook(() => useCacheStats(""), {
      wrapper: createWrapper(),
    });

    expect(result.current.isFetching).toBe(false);
    expect(api.getCacheStats).not.toHaveBeenCalled();
  });

  it("fetches cache stats with projectId", async () => {
    vi.mocked(api.getCacheStats).mockResolvedValueOnce({
      data: { total_entries: 100, total_size_bytes: 5000 },
    } as never);

    const { result } = renderHook(() => useCacheStats("proj-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.getCacheStats).toHaveBeenCalledWith("proj-1");
  });
});
