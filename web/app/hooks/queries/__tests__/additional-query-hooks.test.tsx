import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock the api module
vi.mock("~/lib/api", () => {
  return {
    api: {
      listTeams: vi.fn(),
      getTeam: vi.fn(),
      getHealth: vi.fn(),
    },
  };
});

import { api } from "~/lib/api";
import { useTeams } from "../use-teams";
import { useTeam } from "../use-team";
import { useHealth } from "../use-health";

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

describe("useTeams", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.listTeams and returns response.data", async () => {
    const mockTeams = [
      { id: "t1", name: "Team Alpha" },
      { id: "t2", name: "Team Beta" },
    ];
    vi.mocked(api.listTeams).mockResolvedValueOnce({
      data: mockTeams,
    } as never);

    const { result } = renderHook(() => useTeams(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.listTeams).toHaveBeenCalledTimes(1);
    expect(result.current.data).toEqual(mockTeams);
  });
});

describe("useTeam", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches team when teamId is provided", async () => {
    const mockTeam = { id: "t1", name: "Team Alpha" };
    vi.mocked(api.getTeam).mockResolvedValueOnce({
      data: mockTeam,
    } as never);

    const { result } = renderHook(() => useTeam("t1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.getTeam).toHaveBeenCalledWith("t1");
    expect(result.current.data).toEqual(mockTeam);
  });

  it("is disabled when teamId is undefined", () => {
    const { result } = renderHook(() => useTeam(undefined), {
      wrapper: createWrapper(),
    });

    expect(result.current.isFetching).toBe(false);
    expect(api.getTeam).not.toHaveBeenCalled();
  });

  it("is disabled when teamId is empty string", () => {
    const { result } = renderHook(() => useTeam(""), {
      wrapper: createWrapper(),
    });

    expect(result.current.isFetching).toBe(false);
    expect(api.getTeam).not.toHaveBeenCalled();
  });
});

describe("useHealth", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.getHealth and returns data", async () => {
    const mockHealth = { status: "ok", version: "1.0.0" };
    vi.mocked(api.getHealth).mockResolvedValueOnce(mockHealth as never);

    const { result } = renderHook(() => useHealth(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.getHealth).toHaveBeenCalledTimes(1);
    expect(result.current.data).toEqual(mockHealth);
  });
});
