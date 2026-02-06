import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "./api";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: (failureCount, error) => {
        // Don't retry on 401 or 404
        if (error instanceof ApiError) {
          if (error.status === 401 || error.status === 404) {
            return false;
          }
        }
        return failureCount < 3;
      },
    },
    mutations: {
      retry: false,
    },
  },
});

// Query keys for cache management
export const queryKeys = {
  currentUser: ["currentUser"] as const,
  projects: ["projects"] as const,
  project: (id: string) => ["project", id] as const,
  projectApiKeys: (projectId: string) => ["projectApiKeys", projectId] as const,
  teams: ["teams"] as const,
  team: (id: string) => ["team", id] as const,
  teamMembers: (teamId: string) => ["teamMembers", teamId] as const,
  teamInvitations: (teamId: string) => ["teamInvitations", teamId] as const,
  invitations: ["invitations"] as const,
  runs: (projectId: string, page?: number) =>
    ["runs", projectId, page] as const,
  runDetail: (projectId: string, runId: string) =>
    ["runDetail", projectId, runId] as const,
  githubRepos: ["githubRepos"] as const,
  githubAppInstallations: ["githubAppInstallations"] as const,
  githubAppRepos: (installationId: string) =>
    ["githubAppRepos", installationId] as const,
  projectWorkflows: (projectId: string) =>
    ["projectWorkflows", projectId] as const,
  runWorkflow: (projectId: string, runId: string) =>
    ["runWorkflow", projectId, runId] as const,
};
