import type { Workflow, TaskResult, AnalyticsOverview } from "~/lib/api";
import type { TaskStatusInfo } from "~/components/workflow-dag";

// ── Shared date helpers ─────────────────────────────────────────────

function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

function isoAt(offsetMs: number): string {
  const base = new Date("2026-02-25T10:00:00Z");
  return new Date(base.getTime() + offsetMs).toISOString();
}

export const SHOWCASE_WORKFLOW: Workflow = {
  id: "showcase-workflow",
  name: "ci-pipeline",
  version: 1,
  is_default: true,
  synced_at: "2026-02-25T10:00:00Z",
  tasks: [
    { name: "install", command: "npm ci", group: "setup" },
    {
      name: "lint",
      command: "npm run lint",
      needs: ["install"],
      group: "quality",
    },
    {
      name: "typecheck",
      command: "tsc --noEmit",
      needs: ["install"],
      group: "quality",
    },
    {
      name: "unit-test",
      command: "vitest run",
      needs: ["install"],
      group: "test",
    },
    {
      name: "build",
      command: "npm run build",
      needs: ["lint", "typecheck"],
      group: "build",
    },
    {
      name: "integration-test",
      command: "vitest run --config vitest.integration.ts",
      needs: ["build"],
      group: "test",
      condition: "branch == 'main'",
    },
    {
      name: "docker-build",
      command: "docker build -t app:latest .",
      needs: ["build"],
      group: "build",
    },
    {
      name: "deploy-staging",
      command: "kubectl apply -k overlays/staging",
      needs: ["docker-build", "integration-test"],
    },
    {
      name: "smoke-test",
      command: "playwright test --project=smoke",
      needs: ["deploy-staging"],
      group: "test",
    },
  ],
};

export const SHOWCASE_TASK_STATUSES: Map<string, TaskStatusInfo> = new Map([
  ["install", { status: "cached", duration_ms: 120, cache_hit: true }],
  ["lint", { status: "success", duration_ms: 4320 }],
  ["typecheck", { status: "running", duration_ms: 6810 }],
  ["unit-test", { status: "cached", duration_ms: 80, cache_hit: true }],
  ["build", { status: "success", duration_ms: 12400 }],
  ["integration-test", { status: "running", duration_ms: 8500 }],
  ["docker-build", { status: "success", duration_ms: 18200 }],
  ["deploy-staging", { status: "pending" }],
  ["smoke-test", { status: "pending" }],
]);

const RUN_ID = "showcase-run-001";

export const SHOWCASE_TASK_RESULTS: TaskResult[] = [
  {
    id: "t1",
    run_id: RUN_ID,
    task_name: "install",
    status: "cached",
    exit_code: 0,
    started_at: isoAt(0),
    finished_at: isoAt(120),
    duration_ms: 120,
    cache_hit: true,
    cache_key: "npm-ci-abc123",
  },
  {
    id: "t2",
    run_id: RUN_ID,
    task_name: "lint",
    status: "success",
    exit_code: 0,
    started_at: isoAt(200),
    finished_at: isoAt(4520),
    duration_ms: 4320,
    cache_hit: false,
  },
  {
    id: "t3",
    run_id: RUN_ID,
    task_name: "typecheck",
    status: "running",
    exit_code: 0,
    started_at: isoAt(200),
    finished_at: isoAt(7010),
    duration_ms: 6810,
    cache_hit: false,
  },
  {
    id: "t4",
    run_id: RUN_ID,
    task_name: "unit-test",
    status: "cached",
    exit_code: 0,
    started_at: isoAt(200),
    finished_at: isoAt(280),
    duration_ms: 80,
    cache_hit: true,
    cache_key: "vitest-def456",
  },
  {
    id: "t5",
    run_id: RUN_ID,
    task_name: "build",
    status: "success",
    exit_code: 0,
    started_at: isoAt(7100),
    finished_at: isoAt(19500),
    duration_ms: 12400,
    cache_hit: false,
  },
  {
    id: "t6",
    run_id: RUN_ID,
    task_name: "integration-test",
    status: "running",
    started_at: isoAt(19600),
    duration_ms: 8500,
    cache_hit: false,
  },
  {
    id: "t7",
    run_id: RUN_ID,
    task_name: "docker-build",
    status: "success",
    exit_code: 0,
    started_at: isoAt(19600),
    finished_at: isoAt(37800),
    duration_ms: 18200,
    cache_hit: false,
  },
  {
    id: "t8",
    run_id: RUN_ID,
    task_name: "deploy-staging",
    status: "pending",
    cache_hit: false,
  },
  {
    id: "t9",
    run_id: RUN_ID,
    task_name: "smoke-test",
    status: "pending",
    cache_hit: false,
  },
];

// ── Analytics overview (14-day sinusoidal curves) ───────────────────

function buildChart<T>(days: number, fn: (i: number, date: string) => T): T[] {
  return Array.from({ length: days }, (_, i) => fn(i, daysAgo(days - 1 - i)));
}

const DAYS = 14;

export const SHOWCASE_ANALYTICS: AnalyticsOverview = {
  runs: {
    total_runs: 1247,
    success_runs: 1084,
    failed_runs: 112,
    cancelled_runs: 51,
    success_rate: 86.9,
    avg_duration_ms: 34200,
    chart: buildChart(DAYS, (i, date) => {
      const wave = Math.sin((i / DAYS) * Math.PI * 2);
      const base = 85 + Math.round(wave * 15);
      const success = Math.round(base * 0.87);
      const failed = Math.round(base * 0.09);
      const cancelled = base - success - failed;
      return {
        date,
        success,
        failed,
        cancelled,
        avg_duration_ms: 28000 + Math.round(wave * 8000),
      };
    }),
  },
  cache: {
    total_entries: 342,
    total_size_bytes: 2_147_483_648, // 2 GB
    total_hits: 8941,
    total_misses: 1823,
    hit_rate: 83.1,
    total_bytes_uploaded: 12_884_901_888,
    total_bytes_downloaded: 38_654_705_664,
    chart: buildChart(DAYS, (i, date) => {
      const wave = Math.sin(((i + 2) / DAYS) * Math.PI * 2);
      return {
        date,
        cache_hits: 580 + Math.round(wave * 120),
        cache_misses: 120 + Math.round(wave * 30),
        bytes_uploaded: 800_000_000 + Math.round(wave * 200_000_000),
        bytes_downloaded: 2_400_000_000 + Math.round(wave * 600_000_000),
      };
    }),
  },
  artifacts: {
    total_artifacts: 1893,
    total_size_bytes: 5_368_709_120,
    chart: buildChart(DAYS, (i, date) => {
      const wave = Math.sin(((i + 4) / DAYS) * Math.PI * 2);
      return {
        date,
        count: 125 + Math.round(wave * 30),
        size_bytes: 350_000_000 + Math.round(wave * 80_000_000),
      };
    }),
  },
  bandwidth: {
    total_bytes: 51_539_607_552,
    upload_bytes: 12_884_901_888,
    download_bytes: 38_654_705_664,
    chart: buildChart(DAYS, (i, date) => {
      const wave = Math.sin(((i + 1) / DAYS) * Math.PI * 2);
      return {
        date,
        upload_bytes: 800_000_000 + Math.round(wave * 200_000_000),
        download_bytes: 2_400_000_000 + Math.round(wave * 600_000_000),
      };
    }),
  },
  ai: {
    total_analyses: 423,
    success_analyses: 398,
    failed_analyses: 25,
    total_suggestions: 1247,
    applied_suggestions: 891,
    chart: buildChart(DAYS, (i, date) => {
      const wave = Math.sin(((i + 3) / DAYS) * Math.PI * 2);
      return {
        date,
        analyses: 28 + Math.round(wave * 8),
        suggestions: 82 + Math.round(wave * 20),
      };
    }),
  },
  audit_log: {
    total_events: 5823,
    top_actions: [
      { action: "run.created", count: 1247 },
      { action: "cache.upload", count: 893 },
      { action: "deploy.staging", count: 412 },
      { action: "project.updated", count: 187 },
      { action: "member.invited", count: 54 },
    ],
    top_actors: [
      { actor_email: "alice@acme.dev", count: 2341 },
      { actor_email: "bob@acme.dev", count: 1892 },
      { actor_email: "ci-bot@acme.dev", count: 1590 },
    ],
    chart: buildChart(DAYS, (i, date) => {
      const wave = Math.sin(((i + 5) / DAYS) * Math.PI * 2);
      return {
        date,
        events: 380 + Math.round(wave * 80),
      };
    }),
  },
  projects: [
    {
      project_id: "p1",
      project_name: "web-app",
      total_runs: 623,
      success_rate: 91.2,
      cache_size_bytes: 1_073_741_824,
      artifact_size_bytes: 2_684_354_560,
      bandwidth_bytes: 25_769_803_776,
    },
    {
      project_id: "p2",
      project_name: "api-server",
      total_runs: 412,
      success_rate: 84.5,
      cache_size_bytes: 536_870_912,
      artifact_size_bytes: 1_610_612_736,
      bandwidth_bytes: 15_032_385_536,
    },
    {
      project_id: "p3",
      project_name: "shared-libs",
      total_runs: 212,
      success_rate: 78.3,
      cache_size_bytes: 536_870_912,
      artifact_size_bytes: 1_073_741_824,
      bandwidth_bytes: 10_737_418_240,
    },
  ],
};
