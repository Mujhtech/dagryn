import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { useAuth } from "~/lib/auth";
import { useProject, useRuns, useRunDashboardSummary } from "~/hooks/queries";
import { useTriggerRun } from "~/hooks/mutations";
import { useRunFilters } from "~/hooks/use-url-filters";
import type { RunStatus, TriggerRunRequest } from "~/lib/api";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Icons } from "~/components/icons";
import { ProjectBasicView } from "~/components/projects/project-basic-view";
import { getEventType } from "~/components/projects/run-card";
import { WorkflowDashboard } from "~/components/projects/workflow-dashboard";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute("/_dashboard_layout/projects/$projectId/")(
  {
    component: ProjectDetailPage,
    head: () => {
      return generateMetadata({ title: "Project" });
    },
  },
);

function ProjectDetailPage() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();
  const { user: currentUser } = useAuth();

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

  const [userSearch, setUserSearch] = useState("");
  const [repoSearch, setRepoSearch] = useState("");
  const [selectedRepos] = useState<Set<string>>(new Set());
  const [workflowSearch, setWorkflowSearch] = useState("");
  const [branchSearch, setBranchSearch] = useState("");
  const perPage = 20;

  const [triggerDialogOpen, setTriggerDialogOpen] = useState(false);
  const [triggerTargets, setTriggerTargets] = useState("");
  const [triggerBranch, setTriggerBranch] = useState("");
  const [triggerForce, setTriggerForce] = useState(false);

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
  const { data: runSummary } = useRunDashboardSummary(projectId, 30, true);

  const triggerRunMutation = useTriggerRun(projectId);

  const handleTriggerRun = () => {
    const request: TriggerRunRequest = {};

    if (triggerTargets.trim()) {
      request.targets = triggerTargets
        .split(",")
        .map((target) => target.trim())
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

  const allRuns = runsData?.data ?? [];

  const uniqueUsers = useMemo(
    () =>
      (runSummary?.users ?? []).map((user) => ({
        id: user.id,
        name: user.name,
        avatar: user.avatar_url,
      })),
    [runSummary?.users],
  );

  const uniqueWorkflows = runSummary?.workflows ?? [];
  const uniqueBranches = runSummary?.branches ?? [];
  const dashboardChartData = runSummary?.chart ?? [];

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

  if (projectLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (projectError?.message || !project) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>
            {projectError?.message || "Project not found"}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const latestRunId = allRuns[0]?.id;

  if (project.repo_url) {
    return (
      <WorkflowDashboard
        project={project}
        projectId={projectId}
        chartData={dashboardChartData}
        runs={filteredRuns}
        runsLoading={runsLoading}
        latestRunId={latestRunId}
        page={page}
        setPage={(nextPage) => setPage(nextPage)}
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
        onTriggerRun={handleTriggerRun}
        triggerRunPending={triggerRunMutation.isPending}
        triggerRunErrorMessage={triggerRunMutation.error?.message}
        currentUser={currentUser}
      />
    );
  }

  return (
    <ProjectBasicView
      project={project}
      projectId={projectId}
      filteredRuns={filteredRuns}
      runsLoading={runsLoading}
      page={page}
      setPage={(nextPage) => setPage(nextPage)}
      totalPages={totalPages}
      total={total}
      perPage={perPage}
    />
  );
}
