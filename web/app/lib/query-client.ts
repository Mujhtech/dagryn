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
  runDashboardSummary: (projectId: string, days: number) =>
    ["runDashboardSummary", projectId, days] as const,
  runDetail: (projectId: string, runId: string) =>
    ["runDetail", projectId, runId] as const,
  runArtifacts: (projectId: string, runId: string) =>
    ["runArtifacts", projectId, runId] as const,
  githubRepos: ["githubRepos"] as const,
  githubAppInstallations: ["githubAppInstallations"] as const,
  githubAppRepos: (installationId: string) =>
    ["githubAppRepos", installationId] as const,
  githubWorkflowTranslation: (repoFullName: string, installationId?: string, ref?: string) =>
    ["githubWorkflowTranslation", repoFullName, installationId ?? "none", ref ?? "default"] as const,
  projectWorkflows: (projectId: string) =>
    ["projectWorkflows", projectId] as const,
  runWorkflow: (projectId: string, runId: string) =>
    ["runWorkflow", projectId, runId] as const,
  cacheStats: (projectId: string) => ["cacheStats", projectId] as const,
  cacheAnalytics: (projectId: string, days: number) =>
    ["cacheAnalytics", projectId, days] as const,
  officialPlugins: (q?: string, type?: string, sort?: string) =>
    ["officialPlugins", q ?? "", type ?? "", sort ?? ""] as const,
  plugin: (pluginName: string) => ["plugin", pluginName] as const,
  projectPlugins: (projectId: string) => ["projectPlugins", projectId] as const,
  registryPlugins: (q?: string, type?: string, sort?: string, page?: number) =>
    ["registryPlugins", q ?? "", type ?? "", sort ?? "", page ?? 1] as const,
  registryPlugin: (publisher: string, name: string) =>
    ["registryPlugin", publisher, name] as const,
  registryPluginVersions: (publisher: string, name: string) =>
    ["registryPluginVersions", publisher, name] as const,
  registryPluginAnalytics: (publisher: string, name: string, days: number) =>
    ["registryPluginAnalytics", publisher, name, days] as const,
  featuredPlugins: ["featuredPlugins"] as const,
  trendingPlugins: ["trendingPlugins"] as const,
  publisher: (name: string) => ["publisher", name] as const,
  runAIAnalysis: (projectId: string, runId: string) =>
    ["runAIAnalysis", projectId, runId] as const,
  projectAIAnalyses: (projectId: string, offset?: number) =>
    ["projectAIAnalyses", projectId, offset ?? 0] as const,
  aiSuggestions: (projectId: string, runId: string) =>
    ["aiSuggestions", projectId, runId] as const,
  dashboardOverview: ["dashboardOverview"] as const,
  capabilities: ["capabilities"] as const,
  licenseStatus: ["licenseStatus"] as const,
  sampleTemplate: (language: string) =>
    ["sampleTemplate", language] as const,
};
