import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock the api module
vi.mock("~/lib/api", () => ({
  api: {
    cancelRun: vi.fn(),
    createProject: vi.fn(),
    triggerRun: vi.fn(),
  },
  TriggerRunRequest: {},
}));

import { api } from "~/lib/api";
import { useCancelRun } from "../use-cancel-run";
import { useCreateProject } from "../use-create-project";
import { useTriggerRun } from "../use-trigger-run";

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

describe("useCancelRun", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.cancelRun with correct params", async () => {
    vi.mocked(api.cancelRun).mockResolvedValueOnce({} as never);

    const { result } = renderHook(() => useCancelRun(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        projectId: "p1",
        runId: "r1",
      });
    });

    expect(api.cancelRun).toHaveBeenCalledWith("p1", "r1");
  });

  it("invalidates runDetail and runs queries on success", async () => {
    vi.mocked(api.cancelRun).mockResolvedValueOnce({} as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useCancelRun(), {
      wrapper: createWrapper(),
    });

    // Spy on invalidateQueries after the queryClient is created
    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({ projectId: "p1", runId: "r1" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["runDetail", "p1", "r1"],
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["runs", "p1", undefined],
      }),
    );
  });
});

describe("useCreateProject", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.createProject with input", async () => {
    vi.mocked(api.createProject).mockResolvedValueOnce({
      data: { id: "new-p", name: "My Project" },
    } as never);

    const { result } = renderHook(() => useCreateProject(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        name: "My Project",
        slug: "my-project",
      });
    });

    expect(api.createProject).toHaveBeenCalledWith({
      name: "My Project",
      slug: "my-project",
    });
  });

  it("invalidates projects query on success", async () => {
    vi.mocked(api.createProject).mockResolvedValueOnce({
      data: { id: "new-p" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useCreateProject(), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({
        name: "P",
        slug: "p",
      });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["projects"],
      }),
    );
  });
});

describe("useTriggerRun", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.triggerRun with projectId", async () => {
    vi.mocked(api.triggerRun).mockResolvedValueOnce({
      data: { id: "r1", status: "pending" },
    } as never);

    const { result } = renderHook(() => useTriggerRun("proj-1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({ targets: ["build"] });
    });

    expect(api.triggerRun).toHaveBeenCalledWith("proj-1", {
      targets: ["build"],
    });
  });

  it("calls api.triggerRun without request body", async () => {
    vi.mocked(api.triggerRun).mockResolvedValueOnce({
      data: { id: "r2", status: "pending" },
    } as never);

    const { result } = renderHook(() => useTriggerRun("proj-1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync(undefined);
    });

    expect(api.triggerRun).toHaveBeenCalledWith("proj-1", undefined);
  });

  it("invalidates runs query on success", async () => {
    vi.mocked(api.triggerRun).mockResolvedValueOnce({
      data: { id: "r1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useTriggerRun("proj-1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync(undefined);
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["runs", "proj-1", undefined],
      }),
    );
  });
});
