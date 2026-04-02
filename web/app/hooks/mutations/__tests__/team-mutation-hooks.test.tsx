import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock the api module
vi.mock("~/lib/api", () => ({
  api: {
    createTeam: vi.fn(),
    updateTeam: vi.fn(),
    deleteTeam: vi.fn(),
    createTeamInvitation: vi.fn(),
  },
}));

import { api } from "~/lib/api";
import { useCreateTeam } from "../use-create-team";
import { useUpdateTeam } from "../use-update-team";
import { useDeleteTeam } from "../use-delete-team";
import { useCreateTeamInvitation } from "../use-create-team-invitation";

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

describe("useCreateTeam", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.createTeam with correct input", async () => {
    vi.mocked(api.createTeam).mockResolvedValueOnce({
      data: { id: "t1", name: "My Team" },
    } as never);

    const { result } = renderHook(() => useCreateTeam(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        name: "My Team",
        slug: "my-team",
        description: "A test team",
      });
    });

    expect(api.createTeam).toHaveBeenCalledWith({
      name: "My Team",
      slug: "my-team",
      description: "A test team",
    });
  });

  it("invalidates teams query on success", async () => {
    vi.mocked(api.createTeam).mockResolvedValueOnce({
      data: { id: "t1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useCreateTeam(), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({ name: "Team" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["teams"],
      }),
    );
  });
});

describe("useUpdateTeam", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.updateTeam with teamId and input", async () => {
    vi.mocked(api.updateTeam).mockResolvedValueOnce({
      data: { id: "t1", name: "Updated" },
    } as never);

    const { result } = renderHook(() => useUpdateTeam("t1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        name: "Updated",
        description: "New desc",
      });
    });

    expect(api.updateTeam).toHaveBeenCalledWith("t1", {
      name: "Updated",
      description: "New desc",
    });
  });

  it("invalidates teams and team queries on success", async () => {
    vi.mocked(api.updateTeam).mockResolvedValueOnce({
      data: { id: "t1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useUpdateTeam("t1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({ name: "Updated" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["teams"],
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["team", "t1"],
      }),
    );
  });
});

describe("useDeleteTeam", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.deleteTeam with teamId", async () => {
    vi.mocked(api.deleteTeam).mockResolvedValueOnce(undefined as never);

    const { result } = renderHook(() => useDeleteTeam(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync("t1");
    });

    expect(api.deleteTeam).toHaveBeenCalledWith("t1");
  });

  it("invalidates teams and removes team from cache on success", async () => {
    vi.mocked(api.deleteTeam).mockResolvedValueOnce(undefined as never);
    const invalidateSpy = vi.fn();
    const removeSpy = vi.fn();

    const { result } = renderHook(() => useDeleteTeam(), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;
    removeSpy.mockImplementation(queryClient.removeQueries.bind(queryClient));
    queryClient.removeQueries = removeSpy;

    await act(async () => {
      await result.current.mutateAsync("t1");
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["teams"],
      }),
    );
    expect(removeSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["team", "t1"],
      }),
    );
  });
});

describe("useCreateTeamInvitation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.createTeamInvitation with teamId and input", async () => {
    vi.mocked(api.createTeamInvitation).mockResolvedValueOnce({
      data: { id: "inv1" },
    } as never);

    const { result } = renderHook(() => useCreateTeamInvitation("t1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        email: "user@example.com",
        role: "member",
      });
    });

    expect(api.createTeamInvitation).toHaveBeenCalledWith("t1", {
      email: "user@example.com",
      role: "member",
    });
  });

  it("invalidates teamInvitations query on success", async () => {
    vi.mocked(api.createTeamInvitation).mockResolvedValueOnce({
      data: { id: "inv1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useCreateTeamInvitation("t1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({
        email: "user@example.com",
        role: "member",
      });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["teamInvitations", "t1"],
      }),
    );
  });
});
