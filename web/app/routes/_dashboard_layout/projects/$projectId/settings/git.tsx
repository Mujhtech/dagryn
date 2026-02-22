import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  useProject,
  useGitHubAppInstallations,
  useGitHubAppRepos,
} from "~/hooks/queries";
import { useConnectProjectToGitHub } from "~/hooks/mutations";
import { Button } from "~/components/ui/button";
import { Label } from "~/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Icons } from "~/components/icons";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings/git",
)({
  component: GitSettingsPage,
});

function GitSettingsPage() {
  const { projectId } = Route.useParams();

  const { data: project } = useProject(projectId);

  const isConnected = !!(
    project?.github_repo_id && project?.github_installation_id
  );

  const [showChangeRepo, setShowChangeRepo] = useState(false);
  const [selectedInstallation, setSelectedInstallation] = useState("");
  const [selectedRepo, setSelectedRepo] = useState("");
  const [connectSuccess, setConnectSuccess] = useState(false);

  // Only fetch installations when the selector is shown
  const shouldFetchInstallations = !isConnected || showChangeRepo;
  const { data: installations, isLoading: loadingInstallations } =
    useGitHubAppInstallations();

  const { data: repos, isLoading: loadingRepos } = useGitHubAppRepos(
    selectedInstallation || null,
  );

  const connectToGitHubMutation = useConnectProjectToGitHub(projectId);

  const handleConnect = () => {
    const repo = repos?.find((item) => item.id.toString() === selectedRepo);
    if (!repo || !selectedInstallation) return;

    connectToGitHubMutation.mutate(
      {
        github_installation_id: selectedInstallation,
        github_repo_id: repo.id,
        repo_url: repo.clone_url.replace(".git", ""),
        default_branch: repo.default_branch || undefined,
      },
      {
        onSuccess: () => {
          setConnectSuccess(true);
          setShowChangeRepo(false);
          setSelectedInstallation("");
          setSelectedRepo("");
          setTimeout(() => setConnectSuccess(false), 3000);
        },
      },
    );
  };

  if (!project) return null;

  return (
    <div className="space-y-6">
      <Card className="py-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Icons.Github className="h-5 w-5" />
            GitHub Integration
          </CardTitle>
          <CardDescription>
            Connect this project to a GitHub repository to enable webhook
            triggers.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {connectSuccess ? (
            <div className="rounded-none bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
              Repository connected successfully!
            </div>
          ) : null}

          {isConnected && !showChangeRepo ? (
            <div className="space-y-4">
              <div className="rounded-none bg-green-500/10 p-4 space-y-2">
                <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
                  <Icons.Check className="h-4 w-4" />
                  <span className="font-medium">Connected to GitHub</span>
                </div>
                {project.repo_url ? (
                  <a
                    href={project.repo_url}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
                  >
                    <Icons.Link2 className="h-3 w-3" />
                    {project.repo_url}
                  </a>
                ) : null}
              </div>

              <Button variant="outline" onClick={() => setShowChangeRepo(true)}>
                <Icons.GitBranch className="mr-2 h-4 w-4" />
                Change Repository
              </Button>
            </div>
          ) : (
            <RepoSelector
              installations={
                shouldFetchInstallations ? (installations ?? []) : []
              }
              loadingInstallations={
                shouldFetchInstallations && loadingInstallations
              }
              selectedInstallation={selectedInstallation}
              setSelectedInstallation={setSelectedInstallation}
              repos={repos ?? []}
              loadingRepos={loadingRepos}
              selectedRepo={selectedRepo}
              setSelectedRepo={setSelectedRepo}
              connectError={connectToGitHubMutation.error?.message}
              connectPending={connectToGitHubMutation.isPending}
              onConnect={handleConnect}
              onCancel={
                isConnected ? () => setShowChangeRepo(false) : undefined
              }
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

type RepoSelectorProps = {
  installations: Array<{
    id: string;
    account_login: string;
    account_type: string;
  }>;
  loadingInstallations: boolean;
  selectedInstallation: string;
  setSelectedInstallation: (value: string) => void;
  repos: Array<{ id: number; full_name: string; clone_url: string }>;
  loadingRepos: boolean;
  selectedRepo: string;
  setSelectedRepo: (value: string) => void;
  connectError?: string;
  connectPending: boolean;
  onConnect: () => void;
  onCancel?: () => void;
};

function RepoSelector({
  installations,
  loadingInstallations,
  selectedInstallation,
  setSelectedInstallation,
  repos,
  loadingRepos,
  selectedRepo,
  setSelectedRepo,
  connectError,
  connectPending,
  onConnect,
  onCancel,
}: RepoSelectorProps) {
  if (loadingInstallations) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Icons.Loader className="h-4 w-4 animate-spin" />
        Loading installations...
      </div>
    );
  }

  if (installations.length === 0) {
    return (
      <div className="text-sm text-muted-foreground">
        No GitHub App installations found.{" "}
        <a
          href="https://github.com/apps/dagryn-dev/installations/new"
          target="_blank"
          rel="noreferrer"
          className="text-primary underline"
        >
          Install the Dagryn GitHub App
        </a>{" "}
        to connect repositories.
      </div>
    );
  }

  return (
    <>
      <div className="space-y-2">
        <Label htmlFor="installation">GitHub Installation</Label>
        <Select
          value={selectedInstallation}
          onValueChange={setSelectedInstallation}
        >
          <SelectTrigger id="installation" className="w-full">
            <SelectValue placeholder="Select an installation" />
          </SelectTrigger>
          <SelectContent>
            {installations.map((installation) => (
              <SelectItem key={installation.id} value={installation.id}>
                {installation.account_login} ({installation.account_type})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          <a
            href="https://github.com/apps/dagryn-dev/installations/new"
            target="_blank"
            rel="noreferrer"
            className="text-primary hover:underline inline-flex items-center gap-1"
          >
            <Icons.Github className="h-3 w-3" />
            Add GitHub Account
          </a>
          {" · "}
          <a
            href="https://github.com/settings/installations"
            target="_blank"
            rel="noreferrer"
            className="text-primary hover:underline"
          >
            Configure repositories
          </a>
        </p>
      </div>

      {selectedInstallation ? (
        <div className="space-y-2">
          <Label htmlFor="repo">Repository</Label>
          {loadingRepos ? (
            <div className="flex items-center gap-2 text-muted-foreground">
              <Icons.Loader className="h-4 w-4 animate-spin" />
              Loading repositories...
            </div>
          ) : (
            <Select value={selectedRepo} onValueChange={setSelectedRepo}>
              <SelectTrigger id="repo" className="w-full">
                <SelectValue placeholder="Select a repository" />
              </SelectTrigger>
              <SelectContent>
                {repos.map((repo) => (
                  <SelectItem key={repo.id} value={repo.id.toString()}>
                    {repo.full_name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      ) : null}

      {connectError ? (
        <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
          {connectError}
        </div>
      ) : null}

      <div className="flex items-center gap-2">
        <Button
          onClick={onConnect}
          disabled={!selectedInstallation || !selectedRepo || connectPending}
        >
          {connectPending ? (
            <>
              <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
              Connecting...
            </>
          ) : (
            <>
              <Icons.Github className="mr-2 h-4 w-4" />
              Connect to GitHub
            </>
          )}
        </Button>

        {onCancel ? (
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
      </div>
    </>
  );
}
