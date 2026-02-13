import { useState, useEffect } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useProject, useProjectAPIKeys } from "~/hooks/queries";
import {
  useUpdateProject,
  useDeleteProject,
  useCreateProjectAPIKey,
  useRevokeProjectAPIKey,
  useConnectProjectToGitHub,
} from "~/hooks/mutations";
import { Button } from "~/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { api, type GitHubAppInstallation, type GitHubRepo } from "~/lib/api";
import { Icons } from "~/components/icons";
import { GeneralSettingsCard } from "~/components/projects/settings/general-settings-card";
import { APITokensCard } from "~/components/projects/settings/api-tokens-card";
import { GitHubIntegrationCard } from "~/components/projects/settings/github-integration-card";
import { DangerZoneCard } from "~/components/projects/settings/danger-zone-card";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings",
)({
  component: ProjectSettingsPage,
});

function ProjectSettingsPage() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();

  const {
    data: project,
    isLoading: projectLoading,
    error: projectError,
  } = useProject(projectId);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<"public" | "private">("private");
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState("");
  const [apiKeyName, setApiKeyName] = useState("");
  const [apiKeyExpiry, setApiKeyExpiry] = useState<string>("90d");
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  const [installations, setInstallations] = useState<GitHubAppInstallation[]>([]);
  const [selectedInstallation, setSelectedInstallation] = useState<string>("");
  const [repos, setRepos] = useState<GitHubRepo[]>([]);
  const [selectedRepo, setSelectedRepo] = useState<string>("");
  const [loadingInstallations, setLoadingInstallations] = useState(false);
  const [loadingRepos, setLoadingRepos] = useState(false);
  const [connectSuccess, setConnectSuccess] = useState(false);

  const updateProjectMutation = useUpdateProject(projectId);
  const deleteProjectMutation = useDeleteProject(projectId);
  const createAPIKeyMutation = useCreateProjectAPIKey(projectId);
  const revokeAPIKeyMutation = useRevokeProjectAPIKey(projectId);
  const connectToGitHubMutation = useConnectProjectToGitHub(projectId);

  const {
    data: apiKeys,
    isLoading: apiKeysLoading,
    error: apiKeysError,
  } = useProjectAPIKeys(projectId);

  useEffect(() => {
    if (!project) return;

    setName(project.name || "");
    setDescription(project.description || "");
    setVisibility((project.visibility as "public" | "private") || "private");
  }, [project]);

  useEffect(() => {
    async function loadInstallations() {
      setLoadingInstallations(true);
      try {
        const response = await api.listGitHubAppInstallations();
        setInstallations(response.data);
      } catch {
        // ignore - user may not have GitHub App installed
      } finally {
        setLoadingInstallations(false);
      }
    }

    loadInstallations();
  }, []);

  useEffect(() => {
    if (!selectedInstallation) {
      setRepos([]);
      setSelectedRepo("");
      return;
    }

    async function loadRepos() {
      setLoadingRepos(true);
      try {
        const response = await api.listGitHubAppRepos(selectedInstallation);
        setRepos(response.data);
      } catch {
        setRepos([]);
      } finally {
        setLoadingRepos(false);
      }
    }

    loadRepos();
  }, [selectedInstallation]);

  const handleSave = () => {
    if (!name.trim()) return;

    updateProjectMutation.mutate(
      {
        name: name.trim(),
        description: description.trim() || undefined,
        visibility,
      },
      {
        onSuccess: () => {
          setSaveSuccess(true);
          setTimeout(() => setSaveSuccess(false), 3000);
        },
      },
    );
  };

  const handleDelete = () => {
    deleteProjectMutation.mutate(undefined, {
      onSuccess: () => {
        navigate({ to: "/projects" });
      },
    });
  };

  const handleCreateAPIKey = () => {
    if (!apiKeyName.trim()) return;

    createAPIKeyMutation.mutate(
      {
        name: apiKeyName.trim(),
        expires_in: apiKeyExpiry === "no" ? undefined : apiKeyExpiry,
      },
      {
        onSuccess: (data) => {
          setApiKeyName("");
          setCreatedKey(data.key);
        },
      },
    );
  };

  const handleCopyKey = async () => {
    if (!createdKey) return;
    try {
      await navigator.clipboard.writeText(createdKey);
    } catch {
      // ignore
    }
  };

  const handleConnectToGitHub = () => {
    const repo = repos.find((item) => item.id.toString() === selectedRepo);
    if (!repo || !selectedInstallation) return;

    connectToGitHubMutation.mutate(
      {
        github_installation_id: selectedInstallation,
        github_repo_id: repo.id,
        repo_url: repo.clone_url.replace(".git", ""),
      },
      {
        onSuccess: () => {
          setConnectSuccess(true);
          setSelectedInstallation("");
          setSelectedRepo("");
          setTimeout(() => setConnectSuccess(false), 3000);
        },
      },
    );
  };

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
          <CardDescription>{projectError?.message || "Project not found"}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const canDelete = deleteConfirmText === project.slug;
  const isProjectConnected = !!(project.github_repo_id && project.github_installation_id);

  return (
    <div className="space-y-6 px-6 @container/main py-3 max-w-2xl">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects/$projectId" params={{ projectId }}>
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Project Settings</h1>
          <p className="text-muted-foreground font-mono text-sm">{project.slug}</p>
        </div>
      </div>

      <div className="space-y-6">
        <GeneralSettingsCard
          name={name}
          setName={setName}
          description={description}
          setDescription={setDescription}
          visibility={visibility}
          setVisibility={setVisibility}
          onSave={handleSave}
          isSaving={updateProjectMutation.isPending}
          saveError={updateProjectMutation.error?.message}
          saveSuccess={saveSuccess}
        />

        <APITokensCard
          apiKeys={apiKeys}
          apiKeysLoading={apiKeysLoading}
          apiKeysError={apiKeysError?.message}
          apiKeyName={apiKeyName}
          setApiKeyName={setApiKeyName}
          apiKeyExpiry={apiKeyExpiry}
          setApiKeyExpiry={setApiKeyExpiry}
          createdKey={createdKey}
          onCopyKey={handleCopyKey}
          onCreateToken={handleCreateAPIKey}
          createPending={createAPIKeyMutation.isPending}
          revokePending={revokeAPIKeyMutation.isPending}
          onRevoke={(id) => revokeAPIKeyMutation.mutate(id)}
        />

        <GitHubIntegrationCard
          project={project}
          installations={installations}
          selectedInstallation={selectedInstallation}
          setSelectedInstallation={setSelectedInstallation}
          repos={repos}
          selectedRepo={selectedRepo}
          setSelectedRepo={setSelectedRepo}
          loadingInstallations={loadingInstallations}
          loadingRepos={loadingRepos}
          isConnected={isProjectConnected}
          connectError={connectToGitHubMutation.error?.message}
          connectSuccess={connectSuccess}
          connectPending={connectToGitHubMutation.isPending}
          onConnect={handleConnectToGitHub}
        />

        <DangerZoneCard
          project={project}
          deleteConfirmText={deleteConfirmText}
          setDeleteConfirmText={setDeleteConfirmText}
          canDelete={canDelete}
          deletePending={deleteProjectMutation.isPending}
          deleteError={deleteProjectMutation.error?.message}
          onDelete={handleDelete}
        />
      </div>
    </div>
  );
}
