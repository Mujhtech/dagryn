import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock the api module
vi.mock("~/lib/api", () => ({
  api: {
    activateLicense: vi.fn(),
    updateCurrentUser: vi.fn(),
  },
}));

import { api } from "~/lib/api";
import { useActivateLicense } from "../use-activate-license";
import { useUpdateUser } from "../use-update-user";

let queryClient: QueryClient;

function createWrapper() {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useActivateLicense", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.activateLicense with licenseKey", async () => {
    vi.mocked(api.activateLicense).mockResolvedValueOnce({
      data: { active: true },
    } as never);

    const { result } = renderHook(() => useActivateLicense(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync("LICENSE-KEY-123");
    });

    expect(api.activateLicense).toHaveBeenCalledWith("LICENSE-KEY-123");
  });

  it("invalidates licenseStatus and capabilities queries on success", async () => {
    vi.mocked(api.activateLicense).mockResolvedValueOnce({
      data: { active: true },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useActivateLicense(), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync("LICENSE-KEY-123");
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["licenseStatus"],
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["capabilities"],
      }),
    );
  });
});

describe("useUpdateUser", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.updateCurrentUser with input", async () => {
    vi.mocked(api.updateCurrentUser).mockResolvedValueOnce({
      data: { id: "u1", name: "New Name" },
    } as never);

    const { result } = renderHook(() => useUpdateUser(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({ name: "New Name" });
    });

    expect(api.updateCurrentUser).toHaveBeenCalledWith({ name: "New Name" });
  });

  it("invalidates currentUser query on success", async () => {
    vi.mocked(api.updateCurrentUser).mockResolvedValueOnce({
      data: { id: "u1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useUpdateUser(), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({ name: "Updated" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["currentUser"],
      }),
    );
  });
});
