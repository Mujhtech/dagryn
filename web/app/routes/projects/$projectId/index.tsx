import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { useAuth } from "~/lib/auth";
import { useProject, useRuns, useProjectWorkflows } from "~/hooks/queries";
import { useTriggerRun } from "~/hooks/mutations";
import { useRunFilters } from "~/hooks/use-url-filters";
import type { RunStatus, TriggerRunRequest, Run } from "~/lib/api";
import { WorkflowDag } from "~/components/workflow-dag";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Checkbox } from "~/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { Icons } from "~/components/icons";
import { Avatar, AvatarFallback, AvatarImage } from "~/components/ui/avatar";
import { Progress } from "~/components/ui/progress";
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

export const Route = createFileRoute("/projects/$projectId/")({
  component: ProjectDetailPage,
});

const SCROLLBAR_CLASS =
  "scrollbar-foreground scrollbar-track-transparent scrollbar-thin";

function ProjectDetailPage() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();
  const {
    isAuthenticated,
    isLoading: authLoading,
    user: currentUser,
  } = useAuth();

  // URL-persisted pagination and filter state
  const {
    page,
    setPage,
    statusFilters,
    eventFilters,
    selectedUsers,
    selectedWorkflows,
    selectedBranches,
    toggleStatusFilter,
    toggleEventFilter,
    toggleUser,
    toggleWorkflow,
    toggleBranch,
  } = useRunFilters();

  // Local search state (not persisted in URL)
  const [userSearch, setUserSearch] = useState("");
  const [repoSearch, setRepoSearch] = useState("");
  const [selectedRepos] = useState<Set<string>>(new Set());
  const [workflowSearch, setWorkflowSearch] = useState("");
  const [branchSearch, setBranchSearch] = useState("");
  const perPage = 20;

  // Trigger run dialog state
  const [triggerDialogOpen, setTriggerDialogOpen] = useState(false);
  const [triggerTargets, setTriggerTargets] = useState("");
  const [triggerBranch, setTriggerBranch] = useState("");
  const [triggerForce, setTriggerForce] = useState(false);

  // Use TanStack Query for data fetching
  const {
    data: project,
    isLoading: projectLoading,
    error: projectError,
  } = useProject(projectId);

  const { data: runsData, isLoading: runsLoading } = useRuns(
    projectId,
    page,
    perPage,
  );

  // Mutation for triggering runs
  const triggerRunMutation = useTriggerRun(projectId);

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, authLoading, navigate]);

  const handleTriggerRun = () => {
    const request: TriggerRunRequest = {};

    if (triggerTargets.trim()) {
      request.targets = triggerTargets
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
    }
    if (triggerBranch.trim()) {
      request.git_branch = triggerBranch.trim();
    }
    if (triggerForce) {
      request.force = true;
    }

    triggerRunMutation.mutate(request, {
      onSuccess: (data) => {
        setTriggerDialogOpen(false);
        setTriggerTargets("");
        setTriggerBranch("");
        setTriggerForce(false);
        navigate({
          to: "/projects/$projectId/runs/$runId",
          params: { projectId, runId: data.run_id },
        });
      },
    });
  };

  const loading = authLoading || projectLoading;
  const allRuns = runsData?.data ?? [];

  // Extract unique values for filters
  const uniqueUsers = useMemo(() => {
    const users = new Map<string, { name: string; avatar?: string }>();
    allRuns.forEach((run) => {
      if (run.triggered_by_user) {
        users.set(run.triggered_by_user.id, {
          name: run.triggered_by_user.name,
          avatar: run.triggered_by_user.avatar_url,
        });
      }
      if (run.commit_author_name) {
        const key = run.commit_author_email || run.commit_author_name;
        if (!users.has(key)) {
          users.set(key, {
            name: run.commit_author_name,
            avatar: undefined,
          });
        }
      }
    });
    return Array.from(users.entries()).map(([id, info]) => ({ id, ...info }));
  }, [allRuns]);

  const uniqueWorkflows = useMemo(() => {
    const workflows = new Set<string>();
    allRuns.forEach((run) => {
      if (run.workflow_name) workflows.add(run.workflow_name);
    });
    return Array.from(workflows).sort();
  }, [allRuns]);

  const uniqueBranches = useMemo(() => {
    const branches = new Set<string>();
    allRuns.forEach((run) => {
      if (run.trigger_ref) {
        const branch = run.trigger_ref.replace("refs/heads/", "");
        branches.add(branch);
      }
    });
    return Array.from(branches).sort();
  }, [allRuns]);

  // Apply filters
  const filteredRuns = useMemo(() => {
    return allRuns.filter((run) => {
      if (
        statusFilters.size > 0 &&
        !statusFilters.has(run.status as RunStatus)
      ) {
        return false;
      }

      const eventType = getEventType(run);
      if (eventFilters.size > 0 && !eventFilters.has(eventType)) {
        return false;
      }

      if (selectedUsers.size > 0) {
        const userId =
          run.triggered_by_user?.id ||
          run.commit_author_email ||
          run.commit_author_name ||
          "";
        if (!selectedUsers.has(userId)) {
          return false;
        }
      }

      if (
        selectedWorkflows.size > 0 &&
        !selectedWorkflows.has(run.workflow_name)
      ) {
        return false;
      }

      if (selectedBranches.size > 0) {
        const branch = run.trigger_ref?.replace("refs/heads/", "") || "";
        if (!selectedBranches.has(branch)) {
          return false;
        }
      }

      return true;
    });
  }, [
    allRuns,
    statusFilters,
    eventFilters,
    selectedUsers,
    selectedWorkflows,
    selectedBranches,
  ]);

  const meta = runsData?.meta;
  const totalPages = meta?.total_pages ?? 1;
  const total = meta?.total ?? 0;

  const error = projectError?.message;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !project) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>{error || "Project not found"}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  // Show new dashboard UI when project has repo_url (connected to git platform)
  if (project.repo_url) {
    return (
      <WorkflowDashboard
        project={project}
        projectId={projectId}
        runs={filteredRuns}
        runsLoading={runsLoading}
        page={page}
        setPage={setPage}
        totalPages={totalPages}
        total={total}
        perPage={perPage}
        statusFilters={statusFilters}
        eventFilters={eventFilters}
        userSearch={userSearch}
        setUserSearch={setUserSearch}
        uniqueUsers={uniqueUsers}
        selectedUsers={selectedUsers}
        repoSearch={repoSearch}
        setRepoSearch={setRepoSearch}
        selectedRepos={selectedRepos}
        workflowSearch={workflowSearch}
        setWorkflowSearch={setWorkflowSearch}
        uniqueWorkflows={uniqueWorkflows}
        selectedWorkflows={selectedWorkflows}
        branchSearch={branchSearch}
        setBranchSearch={setBranchSearch}
        uniqueBranches={uniqueBranches}
        selectedBranches={selectedBranches}
        toggleStatusFilter={toggleStatusFilter}
        toggleEventFilter={toggleEventFilter}
        toggleUser={toggleUser}
        toggleWorkflow={toggleWorkflow}
        toggleBranch={toggleBranch}
        triggerDialogOpen={triggerDialogOpen}
        setTriggerDialogOpen={setTriggerDialogOpen}
        triggerTargets={triggerTargets}
        setTriggerTargets={setTriggerTargets}
        triggerBranch={triggerBranch}
        setTriggerBranch={setTriggerBranch}
        triggerForce={triggerForce}
        setTriggerForce={setTriggerForce}
        triggerRunMutation={triggerRunMutation}
        handleTriggerRun={handleTriggerRun}
        currentUser={currentUser}
      />
    );
  }

  // Original simple UI for projects without repo_url
  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects">
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">
              {project.name}
            </h1>
            <Badge
              variant={
                project.visibility === "public" ? "default" : "secondary"
              }
            >
              {project.visibility}
            </Badge>
          </div>
          <p className="text-muted-foreground font-mono">{project.slug}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="icon" asChild>
            <Link to="/projects/$projectId/cache" params={{ projectId }}>
              <Icons.Database className="h-4 w-4" />
            </Link>
          </Button>
          <Button variant="outline" size="icon" asChild>
            <Link to="/projects/$projectId/settings" params={{ projectId }}>
              <Icons.Settings className="h-4 w-4" />
            </Link>
          </Button>
        </div>
      </div>

      {project.description && (
        <Card>
          <CardContent className="pt-6">
            <p className="text-muted-foreground">{project.description}</p>
          </CardContent>
        </Card>
      )}

      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold">Recent Runs</h2>
          <Dialog open={triggerDialogOpen} onOpenChange={setTriggerDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Icons.Play className="mr-2 h-4 w-4" />
                Trigger Run
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Trigger Workflow Run</DialogTitle>
                <DialogDescription>
                  Start a new workflow run for this project.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="targets">Targets (optional)</Label>
                  <Input
                    id="targets"
                    value={triggerTargets}
                    onChange={(e) => setTriggerTargets(e.target.value)}
                    placeholder="build, test, deploy"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="branch">Git Branch (optional)</Label>
                  <Input
                    id="branch"
                    value={triggerBranch}
                    onChange={(e) => setTriggerBranch(e.target.value)}
                    placeholder="main"
                  />
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="force"
                    checked={triggerForce}
                    onCheckedChange={(checked) =>
                      setTriggerForce(checked === true)
                    }
                  />
                  <Label htmlFor="force" className="text-sm font-normal">
                    Force run (ignore cache)
                  </Label>
                </div>
                {triggerRunMutation.error && (
                  <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
                    {triggerRunMutation.error.message}
                  </div>
                )}
              </div>
              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => setTriggerDialogOpen(false)}
                  disabled={triggerRunMutation.isPending}
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleTriggerRun}
                  disabled={triggerRunMutation.isPending}
                >
                  {triggerRunMutation.isPending ? (
                    <>
                      <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                      Triggering...
                    </>
                  ) : (
                    "Trigger Run"
                  )}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {runsLoading ? (
          <div className="flex items-center justify-center py-12">
            <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : filteredRuns.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Icons.Play className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold">No runs yet</h3>
              <p className="text-muted-foreground text-center mt-1 mb-4">
                Run <code className="bg-muted px-1 rounded">dagryn run</code> to
                start your first workflow
              </p>
            </CardContent>
          </Card>
        ) : (
          <>
            <div className="space-y-3">
              {filteredRuns.map((run) => (
                <RunCard
                  key={run.id}
                  run={run}
                  projectId={projectId}
                  repoLabel={
                    project.repo_url
                      ? project.repo_url
                          .replace(/^https?:\/\/(www\.)?github\.com\//, "")
                          .replace(/\.git$/, "")
                      : ""
                  }
                />
              ))}
            </div>
            {totalPages > 1 && (
              <div className="flex items-center justify-between mt-6">
                <p className="text-sm text-muted-foreground">
                  Showing {(page - 1) * perPage + 1} -{" "}
                  {Math.min(page * perPage, total)} of {total} runs
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
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
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    disabled={page === totalPages || runsLoading}
                  >
                    Next
                    <Icons.ChevronRight className="h-4 w-4 ml-1" />
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

// New Workflow Dashboard Component (shown when project has repo_url)
function WorkflowDashboard({
  project,
  projectId,
  runs,
  runsLoading,
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
  triggerRunMutation,
  handleTriggerRun,
  currentUser,
}: {
  project: { id: string; name: string; slug: string; repo_url?: string };
  projectId: string;
  runs: Run[];
  runsLoading: boolean;
  page: number;
  setPage: (p: number) => void;
  totalPages: number;
  total: number;
  perPage: number;
  statusFilters: Set<RunStatus>;
  eventFilters: Set<string>;
  userSearch: string;
  setUserSearch: (s: string) => void;
  uniqueUsers: Array<{ id: string; name: string; avatar?: string }>;
  selectedUsers: Set<string>;
  repoSearch: string;
  setRepoSearch: (s: string) => void;
  selectedRepos: Set<string>; // Not used yet - single repo per project
  workflowSearch: string;
  setWorkflowSearch: (s: string) => void;
  uniqueWorkflows: string[];
  selectedWorkflows: Set<string>;
  branchSearch: string;
  setBranchSearch: (s: string) => void;
  uniqueBranches: string[];
  selectedBranches: Set<string>;
  toggleStatusFilter: (status: RunStatus) => void;
  toggleEventFilter: (event: string) => void;
  toggleUser: (userId: string) => void;
  toggleWorkflow: (workflow: string) => void;
  toggleBranch: (branch: string) => void;
  triggerDialogOpen: boolean;
  setTriggerDialogOpen: (o: boolean) => void;
  triggerTargets: string;
  setTriggerTargets: (s: string) => void;
  triggerBranch: string;
  setTriggerBranch: (s: string) => void;
  triggerForce: boolean;
  setTriggerForce: (b: boolean) => void;
  triggerRunMutation: ReturnType<typeof useTriggerRun>;
  handleTriggerRun: () => void;
  currentUser: {
    id: string;
    name: string;
    email: string;
    avatar_url?: string;
  } | null;
}) {
  // Prepare chart data (group runs by date)
  const chartData = useMemo(() => {
    const dataMap = new Map<
      string,
      {
        date: string;
        success: number;
        failed: number;
        duration: number;
        count: number;
      }
    >();
    runs.forEach((run) => {
      const date = new Date(run.created_at).toLocaleDateString();
      if (!dataMap.has(date)) {
        dataMap.set(date, {
          date,
          success: 0,
          failed: 0,
          duration: 0,
          count: 0,
        });
      }
      const entry = dataMap.get(date)!;
      entry.count++;
      if (run.status === "success") entry.success++;
      if (run.status === "failed") entry.failed++;
      if (run.duration_ms) {
        entry.duration =
          (entry.duration * (entry.count - 1) + run.duration_ms) / entry.count;
      }
    });
    return Array.from(dataMap.values()).sort(
      (a, b) => new Date(a.date).getTime() - new Date(b.date).getTime(),
    );
  }, [runs]);

  // Fetch workflow data
  const { data: workflows } = useProjectWorkflows(projectId);
  const latestWorkflow = workflows?.[0];
  const [workflowExpanded, setWorkflowExpanded] = useState(false);

  const filteredUsers = uniqueUsers.filter((u) =>
    u.name.toLowerCase().includes(userSearch.toLowerCase()),
  );

  const filteredWorkflows = uniqueWorkflows.filter((w) =>
    w.toLowerCase().includes(workflowSearch.toLowerCase()),
  );

  const filteredBranches = uniqueBranches.filter((b) =>
    b.toLowerCase().includes(branchSearch.toLowerCase()),
  );

  // Extract repo name from repo_url
  const repoName = project.repo_url
    ? project.repo_url
        .replace(/^https?:\/\/(www\.)?github\.com\//, "")
        .replace(/\.git$/, "")
    : "";

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Left Sidebar - Filters */}
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
              <FilterCheckbox
                label="Success"
                checked={statusFilters.has("success")}
                onCheckedChange={() => toggleStatusFilter("success")}
                color="text-blue-500"
              />
              <FilterCheckbox
                label="Failed"
                checked={statusFilters.has("failed")}
                onCheckedChange={() => toggleStatusFilter("failed")}
                color="text-pink-500"
              />
              <FilterCheckbox
                label="Cancelled"
                checked={statusFilters.has("cancelled")}
                onCheckedChange={() => toggleStatusFilter("cancelled")}
                color="text-gray-500"
              />
              <FilterCheckbox
                label="Running"
                checked={statusFilters.has("running")}
                onCheckedChange={() => toggleStatusFilter("running")}
                color="text-yellow-500"
              />
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
                  {selectedUsers.has(user.id) && (
                    <Icons.Circle className="h-4 w-4 text-primary" />
                  )}
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
            {repoName && (
              <div className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted">
                <Icons.Github className="h-4 w-4" />
                <span className="text-sm flex-1 text-left">{repoName}</span>
              </div>
            )}
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

      {/* Main Content */}
      <div className={cn("flex-1 overflow-y-auto", SCROLLBAR_CLASS)}>
        <div className="p-6 space-y-6">
          {/* Header */}
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold">{project.name}</h1>
              <p className="text-sm text-muted-foreground font-mono">
                {project.slug}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="icon" asChild>
                <Link to="/projects/$projectId/cache" params={{ projectId }}>
                  <Icons.Database className="h-4 w-4" />
                </Link>
              </Button>
              <Button variant="outline" size="icon" asChild>
                <Link to="/projects/$projectId/settings" params={{ projectId }}>
                  <Icons.Settings className="h-4 w-4" />
                </Link>
              </Button>
              <Dialog
                open={triggerDialogOpen}
                onOpenChange={setTriggerDialogOpen}
              >
                <DialogTrigger asChild>
                  <Button>
                    <Icons.Play className="mr-2 h-4 w-4" />
                    Trigger Run
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Trigger Workflow Run</DialogTitle>
                    <DialogDescription>
                      Start a new workflow run for this project.
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label htmlFor="targets">Targets (optional)</Label>
                      <Input
                        id="targets"
                        value={triggerTargets}
                        onChange={(e) => setTriggerTargets(e.target.value)}
                        placeholder="build, test, deploy"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="branch">Git Branch (optional)</Label>
                      <Input
                        id="branch"
                        value={triggerBranch}
                        onChange={(e) => setTriggerBranch(e.target.value)}
                        placeholder="main"
                      />
                    </div>
                    <div className="flex items-center space-x-2">
                      <Checkbox
                        id="force"
                        checked={triggerForce}
                        onCheckedChange={(checked) =>
                          setTriggerForce(checked === true)
                        }
                      />
                      <Label htmlFor="force" className="text-sm font-normal">
                        Force run (ignore cache)
                      </Label>
                    </div>
                    {triggerRunMutation.error && (
                      <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
                        {triggerRunMutation.error.message}
                      </div>
                    )}
                  </div>
                  <DialogFooter>
                    <Button
                      variant="outline"
                      onClick={() => setTriggerDialogOpen(false)}
                      disabled={triggerRunMutation.isPending}
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={handleTriggerRun}
                      disabled={triggerRunMutation.isPending}
                    >
                      {triggerRunMutation.isPending ? (
                        <>
                          <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                          Triggering...
                        </>
                      ) : (
                        "Trigger Run"
                      )}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {/* Workflow Run Distribution Chart */}
          {chartData.length > 0 && (
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
                  className="h-[300px]"
                >
                  <ResponsiveContainer width="100%" height="100%">
                    <ComposedChart data={chartData}>
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
                        label={{
                          value: "Duration",
                          angle: 90,
                          position: "insideRight",
                        }}
                      />
                      <ChartTooltip
                        content={
                          <ChartTooltipContent
                            // labelFormatter={(value) => {
                            //   return new Date(value).toLocaleDateString(
                            //     "en-US",
                            //     {
                            //       month: "short",
                            //       day: "numeric",
                            //     }
                            //   );
                            // }}
                            indicator="dot"
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
          )}

          {/* Workflow DAG Section */}
          {latestWorkflow && (
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
              {workflowExpanded && (
                <CardContent>
                  <WorkflowDag
                    workflow={latestWorkflow}
                    className="min-h-[200px]"
                  />
                </CardContent>
              )}
            </Card>
          )}

          {/* Runs List */}
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
              {totalPages > 1 && (
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
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// Enhanced Run Card Component
function RunCard({
  run,
  projectId,
  repoLabel,
  currentUser,
}: {
  run: Run;
  projectId: string;
  repoLabel: string;
  currentUser?: {
    id: string;
    name: string;
    email: string;
    avatar_url?: string;
  } | null;
}) {
  // Determine who triggered it and event type
  const triggerInfo = getTriggerInfo(run, currentUser);
  const eventType = getEventType(run);

  // Use PR title for run name if it's a PR, otherwise use workflow name
  const displayName = run.pr_title || run.workflow_name;

  // Commit message or PR description
  const description = run.commit_message || run.pr_title || "";

  // Extract branch from trigger_ref
  const branch = run.trigger_ref?.replace("refs/heads/", "") || "";

  // Progress calculation (simplified - we don't have completed_tasks in Run type yet)
  const progress =
    run.status === "success"
      ? 100
      : run.status === "failed"
        ? 100
        : run.status === "running"
          ? 50
          : 0;

  return (
    <Link
      to="/projects/$projectId/runs/$runId"
      params={{ projectId, runId: run.id }}
      className="block"
    >
      <Card className="hover:border-primary/50 transition-colors cursor-pointer">
        <CardContent className="p-4">
          <div className="flex items-start gap-4">
            {/* Status Icon */}
            <div className="pt-1">
              <RunStatusIcon status={run.status} />
            </div>

            {/* Main Content */}
            <div className="flex-1 min-w-0">
              {/* Workflow/PR Name */}
              <h3 className="font-semibold text-base mb-2">{displayName}</h3>

              {/* User and Event Info */}
              <div className="flex items-center gap-2 mb-2">
                <Avatar className="h-5 w-5">
                  <AvatarImage src={triggerInfo.avatar} />
                  <AvatarFallback className="text-xs">
                    {triggerInfo.name[0]?.toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <span className="text-sm text-muted-foreground">
                  {triggerInfo.name}
                </span>
                <EventIcon eventType={eventType} prNumber={run.pr_number} />
                <span className="text-sm text-muted-foreground">
                  {eventType}
                </span>
                {run.pr_number && (
                  <span className="text-sm text-muted-foreground">
                    #{run.pr_number}
                  </span>
                )}
                <span className="text-sm text-muted-foreground">·</span>
                <span className="text-sm text-muted-foreground">
                  {formatTimeAgo(run.created_at)}
                </span>
              </div>

              {/* Commit Message / PR Description */}
              {description && (
                <p className="text-sm text-muted-foreground mb-3 line-clamp-2">
                  {description}
                </p>
              )}

              {/* Repository and Branch */}
              <div className="flex items-center gap-4 text-sm text-muted-foreground mb-3">
                <div className="flex items-center gap-1">
                  <Icons.Github className="h-4 w-4" />
                  <span>{repoLabel}</span>
                </div>
                {branch && (
                  <div className="flex items-center gap-1">
                    <Icons.GitBranch className="h-4 w-4" />
                    <span>{branch}</span>
                  </div>
                )}
              </div>

              {/* Duration and Progress */}
              <div className="flex items-center gap-4">
                {run.duration_ms != null && (
                  <div className="flex items-center gap-1 text-sm text-muted-foreground">
                    <Icons.Clock className="h-4 w-4" />
                    <span>{formatDuration(run.duration_ms)}</span>
                  </div>
                )}
                {run.task_count > 0 && (
                  <div className="flex-1">
                    <Progress value={progress} className="h-1.5" />
                  </div>
                )}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}

// Helper functions
function getEventType(run: Run): string {
  if (run.pr_number) return "pull_request";
  if (run.trigger_source === "cli") return "push";
  if (run.trigger_source === "ci") return "push";
  if (run.trigger_source === "api" || run.trigger_source === "dashboard")
    return "workflow_dispatch";
  return run.trigger_source || "push";
}

function getTriggerInfo(
  run: Run,
  currentUser?: {
    id: string;
    name: string;
    email: string;
    avatar_url?: string;
  } | null,
): { name: string; avatar?: string } {
  // For git platform runs, use commit author
  if (run.commit_author_name) {
    return {
      name: run.commit_author_name,
      avatar: undefined,
    };
  }

  // For local/API runs, use triggered_by_user or current user
  if (run.triggered_by_user) {
    return {
      name: run.triggered_by_user.name,
      avatar: run.triggered_by_user.avatar_url,
    };
  }

  // Fallback to current user if available
  if (currentUser) {
    return {
      name: currentUser.name,
      avatar: currentUser.avatar_url,
    };
  }

  return { name: "Unknown" };
}

function EventIcon({ eventType }: { eventType: string; prNumber?: number }) {
  switch (eventType) {
    case "pull_request":
      return <Icons.GitPullRequest className="h-4 w-4 text-muted-foreground" />;
    case "schedule":
      return <Icons.Calendar className="h-4 w-4 text-muted-foreground" />;
    case "workflow_dispatch":
      return <Icons.Play className="h-4 w-4 text-muted-foreground" />;
    case "push":
    default:
      return <Icons.GitCommit className="h-4 w-4 text-muted-foreground" />;
  }
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
  icon: React.ReactNode;
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

function RunStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className="h-5 w-5 text-green-500" />;
    case "failed":
      return <Icons.XCircle className="h-5 w-5 text-red-500" />;
    case "running":
      return <Icons.Loader className="h-5 w-5 text-blue-500 animate-spin" />;
    case "pending":
      return <Icons.Circle className="h-5 w-5 text-yellow-500" />;
    case "cancelled":
      return <Icons.XCircle className="h-5 w-5 text-gray-500" />;
    default:
      return <Icons.Circle className="h-5 w-5 text-gray-400" />;
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

function formatTimeAgo(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}
