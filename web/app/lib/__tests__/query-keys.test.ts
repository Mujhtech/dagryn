import { describe, it, expect } from "vitest";
import { queryKeys, queryClient } from "../query-client";
import { ApiError } from "../api";

describe("queryKeys", () => {
  describe("static keys", () => {
    it("currentUser", () => {
      expect(queryKeys.currentUser).toEqual(["currentUser"]);
    });

    it("projects", () => {
      expect(queryKeys.projects).toEqual(["projects"]);
    });

    it("teams", () => {
      expect(queryKeys.teams).toEqual(["teams"]);
    });

    it("invitations", () => {
      expect(queryKeys.invitations).toEqual(["invitations"]);
    });

    it("githubRepos", () => {
      expect(queryKeys.githubRepos).toEqual(["githubRepos"]);
    });

    it("githubAppInstallations", () => {
      expect(queryKeys.githubAppInstallations).toEqual([
        "githubAppInstallations",
      ]);
    });

    it("featuredPlugins", () => {
      expect(queryKeys.featuredPlugins).toEqual(["featuredPlugins"]);
    });

    it("trendingPlugins", () => {
      expect(queryKeys.trendingPlugins).toEqual(["trendingPlugins"]);
    });

    it("dashboardOverview", () => {
      expect(queryKeys.dashboardOverview).toEqual(["dashboardOverview"]);
    });

    it("capabilities", () => {
      expect(queryKeys.capabilities).toEqual(["capabilities"]);
    });

    it("licenseStatus", () => {
      expect(queryKeys.licenseStatus).toEqual(["licenseStatus"]);
    });
  });

  describe("parameterized key factories", () => {
    it("project(id)", () => {
      expect(queryKeys.project("p1")).toEqual(["project", "p1"]);
    });

    it("projectApiKeys(projectId)", () => {
      expect(queryKeys.projectApiKeys("p1")).toEqual(["projectApiKeys", "p1"]);
    });

    it("team(id)", () => {
      expect(queryKeys.team("t1")).toEqual(["team", "t1"]);
    });

    it("teamMembers(teamId)", () => {
      expect(queryKeys.teamMembers("t1")).toEqual(["teamMembers", "t1"]);
    });

    it("teamInvitations(teamId)", () => {
      expect(queryKeys.teamInvitations("t1")).toEqual([
        "teamInvitations",
        "t1",
      ]);
    });

    it("runs(projectId) without page", () => {
      expect(queryKeys.runs("p1")).toEqual(["runs", "p1", undefined]);
    });

    it("runs(projectId, page)", () => {
      expect(queryKeys.runs("p1", 2)).toEqual(["runs", "p1", 2]);
    });

    it("runDashboardSummary(projectId, days)", () => {
      expect(queryKeys.runDashboardSummary("p1", 7)).toEqual([
        "runDashboardSummary",
        "p1",
        7,
      ]);
    });

    it("runDetail(projectId, runId)", () => {
      expect(queryKeys.runDetail("p1", "r1")).toEqual([
        "runDetail",
        "p1",
        "r1",
      ]);
    });

    it("runArtifacts(projectId, runId)", () => {
      expect(queryKeys.runArtifacts("p1", "r1")).toEqual([
        "runArtifacts",
        "p1",
        "r1",
      ]);
    });

    it("githubAppRepos(installationId)", () => {
      expect(queryKeys.githubAppRepos("inst1")).toEqual([
        "githubAppRepos",
        "inst1",
      ]);
    });

    it("githubWorkflowTranslation with all params", () => {
      expect(
        queryKeys.githubWorkflowTranslation("owner/repo", "inst1", "main"),
      ).toEqual(["githubWorkflowTranslation", "owner/repo", "inst1", "main"]);
    });

    it("githubWorkflowTranslation with defaults", () => {
      expect(queryKeys.githubWorkflowTranslation("owner/repo")).toEqual([
        "githubWorkflowTranslation",
        "owner/repo",
        "none",
        "default",
      ]);
    });

    it("projectWorkflows(projectId)", () => {
      expect(queryKeys.projectWorkflows("p1")).toEqual([
        "projectWorkflows",
        "p1",
      ]);
    });

    it("runWorkflow(projectId, runId)", () => {
      expect(queryKeys.runWorkflow("p1", "r1")).toEqual([
        "runWorkflow",
        "p1",
        "r1",
      ]);
    });

    it("cacheStats(projectId)", () => {
      expect(queryKeys.cacheStats("p1")).toEqual(["cacheStats", "p1"]);
    });

    it("cacheAnalytics(projectId, days)", () => {
      expect(queryKeys.cacheAnalytics("p1", 30)).toEqual([
        "cacheAnalytics",
        "p1",
        30,
      ]);
    });

    it("officialPlugins with defaults", () => {
      expect(queryKeys.officialPlugins()).toEqual([
        "officialPlugins",
        "",
        "",
        "",
      ]);
    });

    it("officialPlugins with params", () => {
      expect(queryKeys.officialPlugins("lint", "action", "popular")).toEqual([
        "officialPlugins",
        "lint",
        "action",
        "popular",
      ]);
    });

    it("plugin(pluginName)", () => {
      expect(queryKeys.plugin("eslint")).toEqual(["plugin", "eslint"]);
    });

    it("projectPlugins(projectId)", () => {
      expect(queryKeys.projectPlugins("p1")).toEqual(["projectPlugins", "p1"]);
    });

    it("registryPlugins with defaults", () => {
      expect(queryKeys.registryPlugins()).toEqual([
        "registryPlugins",
        "",
        "",
        "",
        1,
      ]);
    });

    it("registryPlugins with params", () => {
      expect(
        queryKeys.registryPlugins("test", "action", "downloads", 3),
      ).toEqual(["registryPlugins", "test", "action", "downloads", 3]);
    });

    it("registryPlugin(publisher, name)", () => {
      expect(queryKeys.registryPlugin("acme", "lint")).toEqual([
        "registryPlugin",
        "acme",
        "lint",
      ]);
    });

    it("registryPluginVersions(publisher, name)", () => {
      expect(queryKeys.registryPluginVersions("acme", "lint")).toEqual([
        "registryPluginVersions",
        "acme",
        "lint",
      ]);
    });

    it("registryPluginAnalytics(publisher, name, days)", () => {
      expect(queryKeys.registryPluginAnalytics("acme", "lint", 30)).toEqual([
        "registryPluginAnalytics",
        "acme",
        "lint",
        30,
      ]);
    });

    it("publisher(name)", () => {
      expect(queryKeys.publisher("acme")).toEqual(["publisher", "acme"]);
    });

    it("runAIAnalysis(projectId, runId)", () => {
      expect(queryKeys.runAIAnalysis("p1", "r1")).toEqual([
        "runAIAnalysis",
        "p1",
        "r1",
      ]);
    });

    it("projectAIAnalyses with default offset", () => {
      expect(queryKeys.projectAIAnalyses("p1")).toEqual([
        "projectAIAnalyses",
        "p1",
        0,
      ]);
    });

    it("projectAIAnalyses with offset", () => {
      expect(queryKeys.projectAIAnalyses("p1", 10)).toEqual([
        "projectAIAnalyses",
        "p1",
        10,
      ]);
    });

    it("aiSuggestions(projectId, runId)", () => {
      expect(queryKeys.aiSuggestions("p1", "r1")).toEqual([
        "aiSuggestions",
        "p1",
        "r1",
      ]);
    });

    it("teamAuditLogs with default cursor", () => {
      expect(queryKeys.teamAuditLogs("t1")).toEqual([
        "teamAuditLogs",
        "t1",
        "",
      ]);
    });

    it("teamAuditLogs with cursor", () => {
      expect(queryKeys.teamAuditLogs("t1", "abc")).toEqual([
        "teamAuditLogs",
        "t1",
        "abc",
      ]);
    });

    it("projectAuditLogs with default cursor", () => {
      expect(queryKeys.projectAuditLogs("p1")).toEqual([
        "projectAuditLogs",
        "p1",
        "",
      ]);
    });

    it("projectAuditLogs with cursor", () => {
      expect(queryKeys.projectAuditLogs("p1", "xyz")).toEqual([
        "projectAuditLogs",
        "p1",
        "xyz",
      ]);
    });

    it("teamAuditRetention(teamId)", () => {
      expect(queryKeys.teamAuditRetention("t1")).toEqual([
        "teamAuditRetention",
        "t1",
      ]);
    });

    it("teamAuditWebhooks(teamId)", () => {
      expect(queryKeys.teamAuditWebhooks("t1")).toEqual([
        "teamAuditWebhooks",
        "t1",
      ]);
    });

    it("sampleTemplate(language)", () => {
      expect(queryKeys.sampleTemplate("go")).toEqual(["sampleTemplate", "go"]);
    });

    it("teamAnalytics(teamId, days)", () => {
      expect(queryKeys.teamAnalytics("t1", 30)).toEqual([
        "teamAnalytics",
        "t1",
        30,
      ]);
    });

    it("userAnalytics(days)", () => {
      expect(queryKeys.userAnalytics(7)).toEqual(["userAnalytics", 7]);
    });
  });

  describe("query client retry logic", () => {
    it("should not retry on 401 ApiError", () => {
      const retryFn = queryClient.getDefaultOptions().queries?.retry;
      expect(typeof retryFn).toBe("function");

      const error401 = new ApiError(401, "unauthorized", "Unauthorized");
      expect((retryFn as Function)(0, error401)).toBe(false);
    });

    it("should not retry on 404 ApiError", () => {
      const retryFn = queryClient.getDefaultOptions().queries?.retry;

      const error404 = new ApiError(404, "not_found", "Not found");
      expect((retryFn as Function)(0, error404)).toBe(false);
    });

    it("should retry up to 3 times for 5xx errors", () => {
      const retryFn = queryClient.getDefaultOptions().queries
        ?.retry as Function;

      const error500 = new ApiError(500, "server_error", "Internal error");
      expect(retryFn(0, error500)).toBe(true);
      expect(retryFn(1, error500)).toBe(true);
      expect(retryFn(2, error500)).toBe(true);
      expect(retryFn(3, error500)).toBe(false);
    });

    it("should retry non-ApiError errors up to 3 times", () => {
      const retryFn = queryClient.getDefaultOptions().queries
        ?.retry as Function;

      const genericError = new Error("Network error");
      expect(retryFn(0, genericError)).toBe(true);
      expect(retryFn(2, genericError)).toBe(true);
      expect(retryFn(3, genericError)).toBe(false);
    });

    it("should not retry mutations", () => {
      const mutationRetry = queryClient.getDefaultOptions().mutations?.retry;
      expect(mutationRetry).toBe(false);
    });
  });
});
