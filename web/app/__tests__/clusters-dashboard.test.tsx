import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import type { Cluster, Worker, DashboardOverview } from "~/lib/api";

const clustersState: {
  personal: Cluster[];
  byTeam: Record<string, Cluster[]>;
} = {
  personal: [],
  byTeam: {},
};

const workersState: { items: Worker[]; isLoading: boolean } = {
  items: [],
  isLoading: false,
};

const teamsState: { data: { id: string; name?: string }[] } = {
  data: [],
};

const overviewState: { data: DashboardOverview } = {
  data: { projects: [], recent_runs: [] },
};

vi.mock("~/lib/auth", () => ({
  useAuth: () => ({ isAuthenticated: true }),
}));

vi.mock("~/hooks/queries", () => ({
  useTeams: () => ({ data: { data: teamsState.data } }),
  useClusters: (teamId?: string) => ({
    data: teamId ? clustersState.byTeam[teamId] ?? [] : clustersState.personal,
    isLoading: false,
  }),
  useWorkers: () => ({ data: workersState.items, isLoading: workersState.isLoading }),
  useWorkersForAccessibleScopes: () => ({
    data: workersState.items,
    isLoading: workersState.isLoading,
  }),
  useDashboardOverview: () => ({ data: overviewState.data, isLoading: false }),
}));

vi.mock("@tanstack/react-query", async () => {
  const actual = await vi.importActual<typeof import("@tanstack/react-query")>(
    "@tanstack/react-query",
  );
  return {
    ...actual,
    useQueries: ({ queries }: { queries: Array<{ queryKey: readonly unknown[] }> }) =>
      queries.map((query) => {
        const teamId = String(query.queryKey[1] ?? "");
        return { data: clustersState.byTeam[teamId] ?? [], isLoading: false };
      }),
  };
});

vi.mock("@tanstack/react-router", async () => {
  const actual = await vi.importActual<typeof import("@tanstack/react-router")>(
    "@tanstack/react-router",
  );
  return {
    ...actual,
    createFileRoute: () => () => ({}),
    useSearch: () => ({ teamId: undefined }),
    Link: ({ children, to, ...props }: { children: ReactNode; to: string; [key: string]: unknown }) => (
      <a href={to} data-testid="router-link" {...props}>
        {children}
      </a>
    ),
  };
});

import { ClustersPage } from "../clusters";
import { IndexPage } from "../dashboard";

describe("clusters and dashboard routing surfaces", () => {
  beforeEach(() => {
    clustersState.personal = [];
    clustersState.byTeam = {};
    workersState.items = [];
    workersState.isLoading = false;
    teamsState.data = [];
    overviewState.data = { projects: [], recent_runs: [] };
  });

  it("renders cluster list with aggregated worker counts", () => {
    clustersState.personal = [
      {
        id: "c-personal",
        name: "default",
        description: "",
        labels: {},
        scope_type: "personal",
        owner_user_id: "u1",
        slug: "default",
        system_default: true,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-02T00:00:00Z",
      },
    ];
    workersState.items = [
      {
        id: "w1",
        cluster_id: "c-personal",
        hostname: "worker-1",
        status: "online",
        os: "linux",
        arch: "amd64",
        environment: "docker",
        capabilities: [],
        max_concurrent_tasks: 2,
        active_tasks: 0,
        version: "v1",
        labels: {},
        last_heartbeat_at: "2026-01-02T00:00:00Z",
      },
    ];

    render(<ClustersPage />);

    expect(screen.getByText("All Clusters")).toBeInTheDocument();
    expect(screen.getAllByText("default").length).toBeGreaterThan(0);
    expect(screen.getByText("personal")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("renders cluster health card with worker totals and link", () => {
    workersState.items = [
      {
        id: "w1",
        cluster_id: "c1",
        hostname: "worker-online",
        status: "online",
        os: "linux",
        arch: "amd64",
        environment: "docker",
        capabilities: [],
        max_concurrent_tasks: 2,
        active_tasks: 0,
        version: "v1",
        labels: {},
        last_heartbeat_at: "2026-01-02T00:00:00Z",
      },
      {
        id: "w2",
        cluster_id: "c1",
        hostname: "worker-offline",
        status: "offline",
        os: "linux",
        arch: "amd64",
        environment: "docker",
        capabilities: [],
        max_concurrent_tasks: 2,
        active_tasks: 0,
        version: "v1",
        labels: {},
        last_heartbeat_at: "2026-01-02T00:00:00Z",
      },
    ];

    render(<IndexPage />);

    expect(screen.getByText("Cluster Health")).toBeInTheDocument();
    expect(screen.getByText("Total Workers")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("Online")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("degraded")).toBeInTheDocument();
    expect(screen.getByText("View Clusters")).toBeInTheDocument();
  });
});
