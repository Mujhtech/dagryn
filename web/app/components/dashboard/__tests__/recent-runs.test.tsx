import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { RecentRunsSection } from "../recent-runs";
import type { DashboardRun } from "~/lib/api";

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

function makeRun(overrides: Partial<DashboardRun> = {}): DashboardRun {
  return {
    id: "r1",
    project_id: "p1",
    project_name: "My Project",
    workflow_name: "ci",
    status: "success",
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

describe("RecentRunsSection", () => {
  it("shows empty state when no runs", () => {
    render(<RecentRunsSection runs={[]} />);
    expect(screen.getByText("No runs yet")).toBeTruthy();
  });

  it("renders 'Recent Runs' title", () => {
    render(<RecentRunsSection runs={[]} />);
    expect(screen.getByText("Recent Runs")).toBeTruthy();
  });

  it("renders run with project name and workflow", () => {
    const run = makeRun({
      project_name: "backend",
      workflow_name: "deploy",
    });

    render(<RecentRunsSection runs={[run]} />);
    expect(screen.getByText(/backend/)).toBeTruthy();
    expect(screen.getByText("deploy")).toBeTruthy();
  });

  it("renders multiple runs", () => {
    const runs = [
      makeRun({ id: "r1", project_name: "alpha" }),
      makeRun({ id: "r2", project_name: "beta" }),
    ];

    render(<RecentRunsSection runs={runs} />);
    expect(screen.getByText(/alpha/)).toBeTruthy();
    expect(screen.getByText(/beta/)).toBeTruthy();
  });

  it("shows trigger ref as badge when present", () => {
    const run = makeRun({ trigger_ref: "main" });
    render(<RecentRunsSection runs={[run]} />);
    expect(screen.getByText("main")).toBeTruthy();
  });

  it("shows triggered_by_user name when present", () => {
    const run = makeRun({
      triggered_by_user: {
        id: "u1",
        email: "test@test.com",
        name: "Alice",
      },
    });
    render(<RecentRunsSection runs={[run]} />);
    expect(screen.getByText("Alice")).toBeTruthy();
  });

  it("shows commit_author_name when no triggered_by_user", () => {
    const run = makeRun({
      triggered_by_user: undefined,
      commit_author_name: "Bob",
    });
    render(<RecentRunsSection runs={[run]} />);
    expect(screen.getByText("Bob")).toBeTruthy();
  });

  it("shows 'Unknown' when no user or commit author", () => {
    const run = makeRun({
      triggered_by_user: undefined,
      commit_author_name: undefined,
    });
    render(<RecentRunsSection runs={[run]} />);
    expect(screen.getByText("Unknown")).toBeTruthy();
  });

  it("renders links to run detail page", () => {
    const run = makeRun({ id: "r1", project_id: "p1" });
    render(<RecentRunsSection runs={[run]} />);
    const links = screen.getAllByTestId("router-link");
    expect(links.length).toBeGreaterThan(0);
  });
});
