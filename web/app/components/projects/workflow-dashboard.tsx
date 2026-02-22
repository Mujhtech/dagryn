import { Link } from "@tanstack/react-router";
import { useEffect, useState, type ReactNode } from "react";
import { useProjectWorkflows, useRunDetail } from "~/hooks/queries";
import type { Project, Run, RunStatus, TaskStatus, Workflow } from "~/lib/api";
import { WorkflowDag, type TaskStatusInfo } from "~/components/workflow-dag";
import { RunStreamClient, type TaskEventData } from "~/lib/sse";
import { queryClient, queryKeys } from "~/lib/query-client";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Input } from "~/components/ui/input";
import { Checkbox } from "~/components/ui/checkbox";
import { Icons } from "~/components/icons";
import { Avatar, AvatarFallback, AvatarImage } from "~/components/ui/avatar";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import {
  Bar,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  ResponsiveContainer,
  ComposedChart,
} from "recharts";
import { cn } from "~/lib/utils";
import { formatDuration } from "./run-detail/status-ui";
import { RunCard } from "./run-card";
import { TriggerRunDialog } from "./trigger-run-dialog";

const SCROLLBAR_CLASS =
  "scrollbar-foreground scrollbar-track-transparent scrollbar-thin";

const RUN_STATUS_FILTER: Array<{
  label: string;
  value: RunStatus;
  color: string;
}> = [
  { label: "Success", value: "success", color: "text-blue-500" },
  { label: "Failed", value: "failed", color: "text-pink-500" },
  { label: "Cancelled", value: "cancelled", color: "text-gray-500" },
  { label: "Running", value: "running", color: "text-yellow-500" },
];

type CurrentUser = {
  id: string;
  name: string;
  email: string;
  avatar_url?: string;
} | null;

type WorkflowDashboardProps = {
  project: Project;
  projectId: string;
  chartData: Array<{
    date: string;
    success: number;
    failed: number;
    duration_ms: number;
  }>;
  runs: Run[];
  runsLoading: boolean;
  latestRunId?: string;
  page: number;
  setPage: (page: number) => void;
  totalPages: number;
  total: number;
  perPage: number;
  statusFilters: Set<RunStatus>;
  eventFilters: Set<string>;
  userSearch: string;
  setUserSearch: (search: string) => void;
  uniqueUsers: Array<{ id: string; name: string; avatar?: string }>;
  selectedUsers: Set<string>;
  repoSearch: string;
  setRepoSearch: (search: string) => void;
  selectedRepos: Set<string>;
  workflowSearch: string;
  setWorkflowSearch: (search: string) => void;
  uniqueWorkflows: string[];
  selectedWorkflows: Set<string>;
  branchSearch: string;
  setBranchSearch: (search: string) => void;
  uniqueBranches: string[];
  selectedBranches: Set<string>;
  toggleStatusFilter: (status: RunStatus) => void;
  toggleEventFilter: (event: string) => void;
  toggleUser: (userId: string) => void;
  toggleWorkflow: (workflow: string) => void;
  toggleBranch: (branch: string) => void;
  triggerDialogOpen: boolean;
  setTriggerDialogOpen: (open: boolean) => void;
  triggerTargets: string;
  setTriggerTargets: (targets: string) => void;
  triggerBranch: string;
  setTriggerBranch: (branch: string) => void;
  triggerForce: boolean;
  setTriggerForce: (force: boolean) => void;
  onTriggerRun: () => void;
  triggerRunPending: boolean;
  triggerRunErrorMessage?: string;
  currentUser: CurrentUser;
};

export function WorkflowDashboard({
  project,
  projectId,
  chartData,
  runs,
  runsLoading,
  latestRunId,
  page,
  setPage,
  totalPages,
  total,
  perPage,
  statusFilters,
  eventFilters,
  userSearch,
  setUserSearch,
  uniqueUsers,
  selectedUsers,
  repoSearch,
  setRepoSearch,
  selectedRepos,
  workflowSearch,
  setWorkflowSearch,
  uniqueWorkflows,
  selectedWorkflows,
  branchSearch,
  setBranchSearch,
  uniqueBranches,
  selectedBranches,
  toggleStatusFilter,
  toggleEventFilter,
  toggleUser,
  toggleWorkflow,
  toggleBranch,
  triggerDialogOpen,
  setTriggerDialogOpen,
  triggerTargets,
  setTriggerTargets,
  triggerBranch,
  setTriggerBranch,
  triggerForce,
  setTriggerForce,
  onTriggerRun,
  triggerRunPending,
  triggerRunErrorMessage,
  currentUser,
}: WorkflowDashboardProps) {
  const { data: workflows } = useProjectWorkflows(projectId);
  const latestWorkflow = workflows?.[0];
  const [workflowExpanded, setWorkflowExpanded] = useState(false);

  const latestRunStatus = runs[0]?.status;
  const isLatestRunActive =
    latestRunStatus === "running" || latestRunStatus === "pending";

  const { data: latestRunDetail } = useRunDetail(projectId, latestRunId ?? "", {
    refetchInterval: isLatestRunActive ? 3000 : false,
  });

  const [dagTasks, setDagTasks] = useState<Map<string, TaskStatusInfo>>(
    new Map(),
  );

  useEffect(() => {
    const tasks = latestRunDetail?.data?.tasks;
    if (!tasks) {
      setDagTasks(new Map());
      return;
    }
    const map = new Map<string, TaskStatusInfo>();
    for (const task of tasks) {
      map.set(task.task_name, {
        status: task.status,
        duration_ms: task.duration_ms,
        cache_hit: task.cache_hit,
      });
    }
    setDagTasks(map);
  }, [latestRunDetail?.data?.tasks]);

  useEffect(() => {
    if (!isLatestRunActive || !latestRunId) return;

    const client = new RunStreamClient();

    const updateTask = (data: TaskEventData, status: TaskStatus) => {
      setDagTasks((prev) => {
        const next = new Map(prev);
        next.set(data.task_name, {
          status,
          duration_ms: data.duration_ms,
          cache_hit: data.cache_hit,
        });
        return next;
      });
    };

    client.onTaskStarted((data) => updateTask(data, "running"));
    client.onTaskCompleted((data) => updateTask(data, "success"));
    client.onTaskFailed((data) => updateTask(data, "failed"));
    client.onTaskCached((data) => updateTask(data, "cached"));

    const onRunEnd = () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.runs(projectId) });
      queryClient.invalidateQueries({
        queryKey: queryKeys.runDetail(projectId, latestRunId),
      });
    };

    client.onRunCompleted(onRunEnd);
    client.onRunFailed(onRunEnd);
    client.onRunCancelled(onRunEnd);
    client.connect(projectId, latestRunId);

    return () => {
      client.disconnect();
    };
  }, [isLatestRunActive, latestRunId, projectId]);

  useEffect(() => {
    if (isLatestRunActive && latestWorkflow) {
      setWorkflowExpanded(true);
    }
  }, [isLatestRunActive, latestWorkflow]);

  const filteredUsers = uniqueUsers.filter((user) =>
    user.name.toLowerCase().includes(userSearch.toLowerCase()),
  );

  const filteredWorkflows = uniqueWorkflows.filter((workflow) =>
    workflow.toLowerCase().includes(workflowSearch.toLowerCase()),
  );

  const filteredBranches = uniqueBranches.filter((branch) =>
    branch.toLowerCase().includes(branchSearch.toLowerCase()),
  );

  const repoName = project.repo_url
    ? project.repo_url
        .replace(/^https?:\/\/(www\.)?github\.com\//, "")
        .replace(/\.git$/, "")
    : "";

  return (
    <div className="flex h-screen overflow-hidden">
      <div
        className={cn(
          "w-64 border-r bg-muted/30 overflow-y-auto",
          SCROLLBAR_CLASS,
        )}
      >
        <div className="p-4 space-y-6">
          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">
              STATUS
            </h3>
            <div className="space-y-2">
              {RUN_STATUS_FILTER.map((status) => (
                <FilterCheckbox
                  key={status.value}
                  label={status.label}
                  checked={statusFilters.has(status.value)}
                  onCheckedChange={() => toggleStatusFilter(status.value)}
                  color={status.color}
                />
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">
              EVENT
            </h3>
            <div className="flex flex-wrap gap-2">
              <EventFilterButton
                label="Pull Request"
                checked={eventFilters.has("pull_request")}
                onCheckedChange={() => toggleEventFilter("pull_request")}
                icon={<Icons.GitPullRequest className="h-4 w-4" />}
              />
              <EventFilterButton
                label="Schedule"
                checked={eventFilters.has("schedule")}
                onCheckedChange={() => toggleEventFilter("schedule")}
                icon={<Icons.Calendar className="h-4 w-4" />}
              />
              <EventFilterButton
                label="Workflow Dispatch"
                checked={eventFilters.has("workflow_dispatch")}
                onCheckedChange={() => toggleEventFilter("workflow_dispatch")}
                icon={<Icons.Play className="h-4 w-4" />}
              />
              <EventFilterButton
                label="Push"
                checked={eventFilters.has("push")}
                onCheckedChange={() => toggleEventFilter("push")}
                icon={<Icons.GitCommit className="h-4 w-4" />}
              />
            </div>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">
              USERS
            </h3>
            <Input
              placeholder="Search users..."
              value={userSearch}
              onChange={(e) => setUserSearch(e.target.value)}
              className="mb-2 h-8"
            />
            <div
              className={cn(
                "space-y-1 max-h-64 overflow-y-auto",
                SCROLLBAR_CLASS,
              )}
            >
              {filteredUsers.map((user) => (
                <button
                  key={user.id}
                  type="button"
                  onClick={() => toggleUser(user.id)}
                  className="flex items-center gap-2 w-full px-2 py-1.5 rounded hover:bg-muted text-sm"
                >
                  <Avatar className="h-6 w-6">
                    <AvatarImage src={user.avatar} />
                    <AvatarFallback>
                      {user.name[0]?.toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <span className="flex-1 text-left truncate">{user.name}</span>
                  {selectedUsers.has(user.id) ? (
                    <Icons.CheckCircle className="h-4 w-4 text-primary" />
                  ) : null}
                </button>
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">
              REPOSITORIES
            </h3>
            {selectedRepos.size === 0 ? (
              <p className="text-xs text-muted-foreground mb-2">
                No repositories selected.
              </p>
            ) : null}
            <Input
              placeholder="Filter by name..."
              value={repoSearch}
              onChange={(e) => setRepoSearch(e.target.value)}
              className="mb-2 h-8"
            />
            {repoName ? (
              <div className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted">
                <Icons.Github className="h-4 w-4" />
                <span className="text-sm flex-1 text-left">{repoName}</span>
              </div>
            ) : null}
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">
              WORKFLOWS
            </h3>
            {selectedWorkflows.size === 0 ? (
              <p className="text-xs text-muted-foreground mb-2">
                No workflows selected.
              </p>
            ) : null}
            <Input
              placeholder="Filter by name..."
              value={workflowSearch}
              onChange={(e) => setWorkflowSearch(e.target.value)}
              className="mb-2 h-8"
            />
            <div
              className={cn(
                "space-y-1 max-h-48 overflow-y-auto",
                SCROLLBAR_CLASS,
              )}
            >
              {filteredWorkflows.map((workflow) => (
                <label
                  key={workflow}
                  className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted cursor-pointer"
                >
                  <Checkbox
                    checked={selectedWorkflows.has(workflow)}
                    onCheckedChange={() => toggleWorkflow(workflow)}
                  />
                  <span className="text-sm flex-1">{workflow}</span>
                </label>
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase mb-3">
              BRANCH
            </h3>
            {selectedBranches.size === 0 ? (
              <p className="text-xs text-muted-foreground mb-2">
                No branches selected.
              </p>
            ) : null}
            <Input
              placeholder="Filter by name..."
              value={branchSearch}
              onChange={(e) => setBranchSearch(e.target.value)}
              className="mb-2 h-8"
            />
            <div
              className={cn(
                "space-y-1 max-h-48 overflow-y-auto",
                SCROLLBAR_CLASS,
              )}
            >
              {filteredBranches.map((branch) => (
                <label
                  key={branch}
                  className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted cursor-pointer"
                >
                  <Checkbox
                    checked={selectedBranches.has(branch)}
                    onCheckedChange={() => toggleBranch(branch)}
                  />
                  <span className="text-sm flex-1 font-mono">{branch}</span>
                </label>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className={cn("flex-1 overflow-y-auto", SCROLLBAR_CLASS)}>
        <div className="p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold">{project.name}</h1>
              <p className="text-sm text-muted-foreground font-mono">
                {project.slug}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="icon" asChild>
                <Link to="/projects/$projectId/plugins" params={{ projectId }}>
                  <Icons.Package className="h-4 w-4" />
                </Link>
              </Button>
              <Button variant="outline" size="icon" asChild>
                <Link to="/projects/$projectId/cache" params={{ projectId }}>
                  <Icons.Database className="h-4 w-4" />
                </Link>
              </Button>
              <Button variant="outline" size="icon" asChild>
                <Link
                  to="/projects/$projectId/ai-analyses"
                  params={{ projectId }}
                >
                  <Icons.Lightbulb className="h-4 w-4" />
                </Link>
              </Button>
              <Button variant="outline" size="icon" asChild>
                <Link to="/projects/$projectId/settings" params={{ projectId }}>
                  <Icons.Settings className="h-4 w-4" />
                </Link>
              </Button>
              <TriggerRunDialog
                open={triggerDialogOpen}
                onOpenChange={setTriggerDialogOpen}
                triggerTargets={triggerTargets}
                setTriggerTargets={setTriggerTargets}
                triggerBranch={triggerBranch}
                setTriggerBranch={setTriggerBranch}
                triggerForce={triggerForce}
                setTriggerForce={setTriggerForce}
                onTriggerRun={onTriggerRun}
                isPending={triggerRunPending}
                errorMessage={triggerRunErrorMessage}
                defaultBranch={project.default_branch}
              />
            </div>
          </div>

          {chartData.length > 0 ? (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-semibold uppercase">
                  WORKFLOW RUN DISTRIBUTION
                </CardTitle>
              </CardHeader>
              <CardContent>
                <ChartContainer
                  config={{
                    success: { color: "hsl(var(--chart-1))" },
                    failed: { color: "hsl(var(--chart-2))" },
                    duration: { color: "hsl(var(--chart-3))" },
                  }}
                  className="h-75"
                >
                  <ResponsiveContainer width="100%" height="100%">
                    <ComposedChart
                      data={chartData.map((point) => ({
                        date: point.date,
                        success: point.success,
                        failed: point.failed,
                        duration: point.duration_ms,
                      }))}
                    >
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="date" />
                      <YAxis
                        yAxisId="left"
                        label={{
                          value: "Run Count",
                          angle: -90,
                          position: "insideLeft",
                        }}
                      />
                      <YAxis
                        yAxisId="right"
                        orientation="right"
                        tickFormatter={(v: number) => formatDuration(v)}
                        label={{
                          value: "Duration",
                          angle: 90,
                          position: "insideRight",
                        }}
                      />
                      <ChartTooltip
                        content={
                          <ChartTooltipContent
                            indicator="dot"
                            formatter={(value, name) =>
                              name === "duration"
                                ? formatDuration(value as number)
                                : value
                            }
                          />
                        }
                      />
                      <Bar
                        yAxisId="left"
                        dataKey="success"
                        stackId="a"
                        fill="var(--color-success)"
                      />
                      <Bar
                        yAxisId="left"
                        dataKey="failed"
                        stackId="a"
                        fill="var(--color-failed)"
                      />
                      <Line
                        yAxisId="right"
                        type="monotone"
                        dataKey="duration"
                        stroke="var(--color-duration)"
                        strokeDasharray="5 5"
                      />
                    </ComposedChart>
                  </ResponsiveContainer>
                </ChartContainer>
              </CardContent>
            </Card>
          ) : null}

          {latestWorkflow ? (
            <Card>
              <CardHeader
                className="cursor-pointer"
                onClick={() => setWorkflowExpanded(!workflowExpanded)}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Icons.Network className="h-4 w-4 text-muted-foreground" />
                    <CardTitle className="text-sm font-semibold uppercase">
                      WORKFLOW DAG
                    </CardTitle>
                    <Badge variant="secondary" className="text-xs">
                      {latestWorkflow.name} v{latestWorkflow.version}
                    </Badge>
                  </div>
                  {workflowExpanded ? (
                    <Icons.ChevronUp className="h-4 w-4 text-muted-foreground" />
                  ) : (
                    <Icons.ChevronDown className="h-4 w-4 text-muted-foreground" />
                  )}
                </div>
              </CardHeader>
              {workflowExpanded ? (
                <CardContent>
                  <WorkflowDag
                    workflow={latestWorkflow}
                    taskStatuses={dagTasks.size > 0 ? dagTasks : undefined}
                    className="min-h-50"
                  />
                </CardContent>
              ) : null}
            </Card>
          ) : null}

          {latestWorkflow &&
          (latestWorkflow.cache ||
            latestWorkflow.ai ||
            latestWorkflow.container) ? (
            <WorkflowConfigBadges workflow={latestWorkflow} />
          ) : null}

          {runsLoading ? (
            <div className="flex items-center justify-center py-12">
              <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : runs.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-12">
                <Icons.Play className="h-12 w-12 text-muted-foreground mb-4" />
                <h3 className="text-lg font-semibold">No runs yet</h3>
                <p className="text-muted-foreground text-center mt-1 mb-4">
                  Runs will appear here once triggered
                </p>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="space-y-2">
                {runs.map((run) => {
                  const repoLabel = project.repo_url
                    ? project.repo_url
                        .replace(/^https?:\/\/[^/]+\//, "")
                        .replace(/\.git$/, "")
                    : project.id;

                  return (
                    <RunCard
                      key={run.id}
                      run={run}
                      projectId={projectId}
                      repoLabel={repoLabel}
                      currentUser={currentUser}
                    />
                  );
                })}
              </div>
              {totalPages > 1 ? (
                <div className="flex items-center justify-between mt-4">
                  <p className="text-sm text-muted-foreground">
                    Showing {(page - 1) * perPage + 1} -{" "}
                    {Math.min(page * perPage, total)} of {total} runs. Use the
                    date range and other filters to narrow your search.
                  </p>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(Math.max(1, page - 1))}
                      disabled={page === 1 || runsLoading}
                    >
                      <Icons.ChevronLeft className="h-4 w-4 mr-1" />
                      Previous
                    </Button>
                    <div className="flex items-center gap-1 px-2">
                      <span className="text-sm font-medium">{page}</span>
                      <span className="text-sm text-muted-foreground">of</span>
                      <span className="text-sm font-medium">{totalPages}</span>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(Math.min(totalPages, page + 1))}
                      disabled={page === totalPages || runsLoading}
                    >
                      Next
                      <Icons.ChevronRight className="h-4 w-4 ml-1" />
                    </Button>
                  </div>
                </div>
              ) : null}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function FilterCheckbox({
  label,
  checked,
  onCheckedChange,
  color,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: () => void;
  color?: string;
}) {
  return (
    <label className="flex items-center gap-2 cursor-pointer">
      <Checkbox checked={checked} onCheckedChange={onCheckedChange} />
      <div className="flex items-center gap-2">
        <Icons.Circle className={`h-3 w-3 fill-current ${color || ""}`} />
        <span className="text-sm">{label}</span>
      </div>
    </label>
  );
}

function EventFilterButton({
  label,
  checked,
  onCheckedChange,
  icon,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: () => void;
  icon: ReactNode;
}) {
  return (
    <Button
      type="button"
      variant={checked ? "default" : "outline"}
      size="sm"
      onClick={onCheckedChange}
      className="h-8"
    >
      {icon}
      <span className="ml-1">{label}</span>
    </Button>
  );
}

function WorkflowConfigBadges({ workflow }: { workflow: Workflow }) {
  const badges: ReactNode[] = [];

  if (workflow.cache) {
    const c = workflow.cache;
    if (c.enabled) {
      badges.push(
        <Badge key="cache-local" variant="outline" className="gap-1">
          <Icons.Database className="h-3 w-3" />
          Cache: Local
        </Badge>,
      );
    }
    if (c.remote_cloud) {
      badges.push(
        <Badge key="cache-cloud" variant="outline" className="gap-1">
          <Icons.Cloud className="h-3 w-3" />
          Cache: Cloud
        </Badge>,
      );
    } else if (c.remote_enabled) {
      badges.push(
        <Badge key="cache-remote" variant="outline" className="gap-1">
          <Icons.Cloud className="h-3 w-3" />
          Cache: Remote
        </Badge>,
      );
    }
  }

  if (workflow.ai?.enabled) {
    const parts = [workflow.ai.mode, workflow.ai.provider, workflow.ai.model]
      .filter(Boolean)
      .join(" / ");
    badges.push(
      <Badge key="ai" variant="outline" className="gap-1">
        <Icons.Lightbulb className="h-3 w-3" />
        AI{parts ? `: ${parts}` : ""}
      </Badge>,
    );
  }

  if (workflow.container?.enabled) {
    const c = workflow.container;
    const parts = [
      c.image,
      c.memory_limit ? `mem:${c.memory_limit}` : "",
      c.cpu_limit ? `cpu:${c.cpu_limit}` : "",
    ]
      .filter(Boolean)
      .join(" / ");
    badges.push(
      <Badge key="container" variant="outline" className="gap-1">
        <Icons.Box className="h-3 w-3" />
        Container{parts ? `: ${parts}` : ""}
      </Badge>,
    );
  }

  if (badges.length === 0) return null;

  return <div className="flex flex-wrap gap-2">{badges}</div>;
}
