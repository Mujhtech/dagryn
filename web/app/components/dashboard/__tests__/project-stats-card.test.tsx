import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProjectStatsCard } from "../project-stats-card";
import type { DashboardProject } from "~/lib/api";

// Mock @tanstack/react-router
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    ...props
  }: {
    children: React.ReactNode;
    to: string;
    [key: string]: unknown;
  }) => (
    <a href={to} data-testid="router-link" {...props}>
      {children}
    </a>
  ),
}));

function makeProject(
  overrides: Partial<DashboardProject> = {},
): DashboardProject {
  return {
    id: "p1",
    name: "Test Project",
    slug: "test-project",
    visibility: "private",
    member_count: 3,
    updated_at: "2025-01-01T00:00:00Z",
    created_at: "2024-01-01T00:00:00Z",
    chart: [],
    total_runs_7d: 0,
    success_runs_7d: 0,
    failed_runs_7d: 0,
    avg_duration_ms: 0,
    ...overrides,
  };
}

describe("ProjectStatsCard", () => {
  it("renders project name", () => {
    render(<ProjectStatsCard project={makeProject({ name: "My App" })} />);
    expect(screen.getByText("My App")).toBeTruthy();
  });

  it("renders project slug", () => {
    render(
      <ProjectStatsCard project={makeProject({ slug: "my-app" })} />,
    );
    expect(screen.getByText("my-app")).toBeTruthy();
  });

  it("renders visibility badge", () => {
    render(
      <ProjectStatsCard project={makeProject({ visibility: "public" })} />,
    );
    expect(screen.getByText("public")).toBeTruthy();
  });

  it("shows '--' placeholder when zero runs", () => {
    render(
      <ProjectStatsCard project={makeProject({ total_runs_7d: 0 })} />,
    );
    const dashes = screen.getAllByText("--");
    // Speed, Reliability, and Builds should all show "--"
    expect(dashes.length).toBe(3);
  });

  it("calculates reliability percentage correctly", () => {
    render(
      <ProjectStatsCard
        project={makeProject({
          total_runs_7d: 10,
          success_runs_7d: 8,
          avg_duration_ms: 5000,
        })}
      />,
    );
    expect(screen.getByText("80%")).toBeTruthy();
  });

  it("shows 100% reliability when all runs succeed", () => {
    render(
      <ProjectStatsCard
        project={makeProject({
          total_runs_7d: 5,
          success_runs_7d: 5,
          avg_duration_ms: 1000,
        })}
      />,
    );
    expect(screen.getByText("100%")).toBeTruthy();
  });

  it("shows runs per week", () => {
    render(
      <ProjectStatsCard
        project={makeProject({
          total_runs_7d: 42,
          success_runs_7d: 40,
          avg_duration_ms: 1000,
        })}
      />,
    );
    expect(screen.getByText("42/wk")).toBeTruthy();
  });

  it("strips GitHub URL prefix from repo_url", () => {
    render(
      <ProjectStatsCard
        project={makeProject({
          repo_url: "https://github.com/acme/backend",
        })}
      />,
    );
    expect(screen.getByText("acme/backend")).toBeTruthy();
  });

  it("strips .git suffix from repo_url", () => {
    render(
      <ProjectStatsCard
        project={makeProject({
          repo_url: "https://github.com/acme/backend.git",
        })}
      />,
    );
    expect(screen.getByText("acme/backend")).toBeTruthy();
  });

  it("shows branch when top_branch is set", () => {
    render(
      <ProjectStatsCard
        project={makeProject({ top_branch: "develop" })}
      />,
    );
    expect(screen.getByText("develop")).toBeTruthy();
  });

  it("does not show repo info when no repo_url", () => {
    render(
      <ProjectStatsCard
        project={makeProject({ repo_url: undefined })}
      />,
    );
    expect(screen.queryByText("acme/backend")).toBeNull();
  });

  it("renders link to project page", () => {
    render(<ProjectStatsCard project={makeProject({ id: "proj-42" })} />);
    const link = screen.getByTestId("router-link");
    expect(link).toBeTruthy();
  });

  it("formats average duration", () => {
    render(
      <ProjectStatsCard
        project={makeProject({
          total_runs_7d: 5,
          success_runs_7d: 5,
          avg_duration_ms: 90000, // 1m 30s
        })}
      />,
    );
    expect(screen.getByText("1m 30s")).toBeTruthy();
  });
});
