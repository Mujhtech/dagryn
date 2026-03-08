import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock the api module
vi.mock("~/lib/api", () => ({
  api: {
    updateProject: vi.fn(),
    deleteProject: vi.fn(),
    connectProjectToGitHub: vi.fn(),
    createProjectAPIKey: vi.fn(),
    revokeProjectAPIKey: vi.fn(),
  },
}));

import { api } from "~/lib/api";
import { useUpdateProject } from "../use-update-project";
import { useDeleteProject } from "../use-delete-project";
import { useConnectProjectToGitHub } from "../use-connect-project-to-github";
import { useCreateProjectAPIKey } from "../use-create-project-api-key";
import { useRevokeProjectAPIKey } from "../use-revoke-project-api-key";

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

describe("useUpdateProject", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.updateProject with projectId and input", async () => {
    vi.mocked(api.updateProject).mockResolvedValueOnce({
      data: { id: "p1", name: "Updated" },
    } as never);

    const { result } = renderHook(() => useUpdateProject("p1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        name: "Updated",
        description: "New desc",
        visibility: "private",
      });
    });

    expect(api.updateProject).toHaveBeenCalledWith("p1", {
      name: "Updated",
      description: "New desc",
      visibility: "private",
    });
  });

  it("invalidates project and projects queries on success", async () => {
    vi.mocked(api.updateProject).mockResolvedValueOnce({
      data: { id: "p1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useUpdateProject("p1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({ name: "Updated" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["project", "p1"],
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["projects"],
      }),
    );
  });
});

describe("useDeleteProject", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.deleteProject with projectId", async () => {
    vi.mocked(api.deleteProject).mockResolvedValueOnce(undefined as never);

    const { result } = renderHook(() => useDeleteProject("p1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(api.deleteProject).toHaveBeenCalledWith("p1");
  });

  it("invalidates projects and removes project from cache on success", async () => {
    vi.mocked(api.deleteProject).mockResolvedValueOnce(undefined as never);
    const invalidateSpy = vi.fn();
    const removeSpy = vi.fn();

    const { result } = renderHook(() => useDeleteProject("p1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;
    removeSpy.mockImplementation(queryClient.removeQueries.bind(queryClient));
    queryClient.removeQueries = removeSpy;

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["projects"],
      }),
    );
    expect(removeSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["project", "p1"],
      }),
    );
  });
});

describe("useConnectProjectToGitHub", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.connectProjectToGitHub with projectId and data", async () => {
    vi.mocked(api.connectProjectToGitHub).mockResolvedValueOnce({
      data: {},
    } as never);

    const { result } = renderHook(() => useConnectProjectToGitHub("p1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        github_installation_id: "inst-1",
        github_repo_id: 12345,
        repo_url: "https://github.com/org/repo",
      });
    });

    expect(api.connectProjectToGitHub).toHaveBeenCalledWith("p1", {
      github_installation_id: "inst-1",
      github_repo_id: 12345,
      repo_url: "https://github.com/org/repo",
    });
  });

  it("invalidates project and projects queries on success", async () => {
    vi.mocked(api.connectProjectToGitHub).mockResolvedValueOnce({
      data: {},
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useConnectProjectToGitHub("p1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({
        github_installation_id: "inst-1",
        github_repo_id: 12345,
        repo_url: "https://github.com/org/repo",
      });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["project", "p1"],
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["projects"],
      }),
    );
  });
});

describe("useCreateProjectAPIKey", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.createProjectAPIKey with projectId and input", async () => {
    vi.mocked(api.createProjectAPIKey).mockResolvedValueOnce({
      data: { id: "key-1", name: "CI Key" },
    } as never);

    const { result } = renderHook(() => useCreateProjectAPIKey("p1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        name: "CI Key",
        expires_in: "30d",
      });
    });

    expect(api.createProjectAPIKey).toHaveBeenCalledWith("p1", {
      name: "CI Key",
      expires_in: "30d",
    });
  });

  it("invalidates projectApiKeys query on success", async () => {
    vi.mocked(api.createProjectAPIKey).mockResolvedValueOnce({
      data: { id: "key-1" },
    } as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useCreateProjectAPIKey("p1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync({ name: "Key" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["projectApiKeys", "p1"],
      }),
    );
  });
});

describe("useRevokeProjectAPIKey", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls api.revokeProjectAPIKey with projectId and keyId", async () => {
    vi.mocked(api.revokeProjectAPIKey).mockResolvedValueOnce(undefined as never);

    const { result } = renderHook(() => useRevokeProjectAPIKey("p1"), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync("key-1");
    });

    expect(api.revokeProjectAPIKey).toHaveBeenCalledWith("p1", "key-1");
  });

  it("invalidates projectApiKeys query on success", async () => {
    vi.mocked(api.revokeProjectAPIKey).mockResolvedValueOnce(undefined as never);
    const invalidateSpy = vi.fn();

    const { result } = renderHook(() => useRevokeProjectAPIKey("p1"), {
      wrapper: createWrapper(),
    });

    invalidateSpy.mockImplementation(queryClient.invalidateQueries.bind(queryClient));
    queryClient.invalidateQueries = invalidateSpy;

    await act(async () => {
      await result.current.mutateAsync("key-1");
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ["projectApiKeys", "p1"],
      }),
    );
  });
});
