import { useState, useEffect } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { FolderCog } from "lucide-react";

import { useAuth } from "~/lib/auth";
import { useProject, useProjectAPIKeys } from "~/hooks/queries";
import {
  useUpdateProject,
  useDeleteProject,
  useCreateProjectAPIKey,
  useRevokeProjectAPIKey,
  useConnectProjectToGitHub,
} from "~/hooks/mutations";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
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
  CardFooter,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "~/components/ui/alert-dialog";
import { Separator } from "~/components/ui/separator";
import { Badge } from "~/components/ui/badge";
import { api, type GitHubAppInstallation, type GitHubRepo } from "~/lib/api";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/projects/$projectId/settings")({
  component: ProjectSettingsPage,
});

function ProjectSettingsPage() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  // Fetch project data
  const {
    data: project,
    isLoading: projectLoading,
    error: projectError,
  } = useProject(projectId);

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<"public" | "private">("private");
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState("");
  const [apiKeyName, setApiKeyName] = useState("");
  const [apiKeyExpiry, setApiKeyExpiry] = useState<string>("90d");
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  // GitHub connection state
  const [installations, setInstallations] = useState<GitHubAppInstallation[]>(
    [],
  );
  const [selectedInstallation, setSelectedInstallation] = useState<string>("");
  const [repos, setRepos] = useState<GitHubRepo[]>([]);
  const [selectedRepo, setSelectedRepo] = useState<string>("");
  const [loadingInstallations, setLoadingInstallations] = useState(false);
  const [loadingRepos, setLoadingRepos] = useState(false);
  const [connectSuccess, setConnectSuccess] = useState(false);

  // Mutations
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

  // Redirect if not authenticated
  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, authLoading, navigate]);

  // Initialize form with project data
  useEffect(() => {
    if (project) {
      setName(project.name || "");
      setDescription(project.description || "");
      setVisibility((project.visibility as "public" | "private") || "private");
    }
  }, [project]);

  // Load GitHub installations on mount
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

  // Load repos when installation changes
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

  const loading = authLoading || projectLoading;
  const error = projectError?.message;

  const handleSave = async () => {
    if (!name.trim()) {
      return;
    }

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

  const handleDelete = async () => {
    deleteProjectMutation.mutate(undefined, {
      onSuccess: () => {
        navigate({ to: "/projects" });
      },
    });
  };

  const handleCreateAPIKey = async () => {
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

  const canDelete = deleteConfirmText === project?.slug;

  const isProjectConnected =
    project?.github_repo_id && project?.github_installation_id;

  const handleConnectToGitHub = async () => {
    const repo = repos.find((r) => r.id.toString() === selectedRepo);
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

  return (
    <div className="space-y-6 px-6 @container/main py-3 max-w-2xl">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects/$projectId" params={{ projectId }}>
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Project Settings
          </h1>
          <p className="text-muted-foreground font-mono text-sm">
            {project.slug}
          </p>
        </div>
      </div>

      <div className="space-y-6">
        {/* General Settings Card */}
        <Card className="py-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <FolderCog className="h-5 w-5" />
              General
            </CardTitle>
            <CardDescription>
              Update your project&apos;s basic information.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Project Name */}
            <div className="space-y-2">
              <Label htmlFor="name">Project Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="My Project"
              />
            </div>

            {/* Description */}
            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="A brief description of your project"
                className="resize-none"
                rows={3}
              />
            </div>

            {/* Visibility */}
            <div className="space-y-2">
              <Label htmlFor="visibility">Visibility</Label>
              <Select
                value={visibility}
                onValueChange={(value) =>
                  setVisibility(value as "public" | "private")
                }
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select visibility" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="private">
                    Private - Only you and team members can see this project
                  </SelectItem>
                  <SelectItem value="public">
                    Public - Anyone can see this project
                  </SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {visibility === "public"
                  ? "Public projects are visible to everyone."
                  : "Private projects are only visible to you and your team."}
              </p>
            </div>

            {/* Error/Success Messages */}
            {updateProjectMutation.error && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {updateProjectMutation.error.message}
              </div>
            )}

            {saveSuccess && (
              <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
                Project settings updated successfully!
              </div>
            )}
          </CardContent>
          <CardFooter>
            <Button
              onClick={handleSave}
              disabled={updateProjectMutation.isPending || !name.trim()}
            >
              {updateProjectMutation.isPending ? (
                <>
                  <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Icons.FloppyDisk className="mr-2 h-4 w-4" />
                  Save Changes
                </>
              )}
            </Button>
          </CardFooter>
        </Card>

        {/* API Tokens Card */}
        <Card className="py-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Icons.Key className="h-5 w-5" />
              API Tokens
            </CardTitle>
            <CardDescription>
              Create and manage API tokens for CI/CD and other integrations.
              Tokens are scoped to this project.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {apiKeysError && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {apiKeysError.message}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="api-key-name">Token name</Label>
              <Input
                id="api-key-name"
                value={apiKeyName}
                onChange={(e) => setApiKeyName(e.target.value)}
                placeholder="Production deploy token"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="api-key-expiry">Expiration</Label>
              <Select
                value={apiKeyExpiry}
                onValueChange={(value) => setApiKeyExpiry(value)}
              >
                <SelectTrigger id="api-key-expiry" className="w-full">
                  <SelectValue placeholder="Select expiration" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="30d">30 days</SelectItem>
                  <SelectItem value="90d">90 days</SelectItem>
                  <SelectItem value="1y">1 year</SelectItem>
                  <SelectItem value="no">No expiration</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                You can revoke a token at any time. For most CI systems,{" "}
                <span className="font-medium">90 days</span> is a good default.
              </p>
            </div>

            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <Button
                onClick={handleCreateAPIKey}
                disabled={createAPIKeyMutation.isPending || !apiKeyName.trim()}
              >
                {createAPIKeyMutation.isPending ? (
                  <>
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  "Create Token"
                )}
              </Button>

              {createdKey && (
                <div className="flex flex-1 items-center gap-2 rounded-md bg-muted px-3 py-2">
                  <Icons.Key className="h-4 w-4 text-muted-foreground" />
                  <span className="font-mono text-xs truncate">
                    {createdKey}
                  </span>
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    className="h-7 w-7"
                    onClick={handleCopyKey}
                  >
                    <Icons.Copy className="h-3 w-3" />
                  </Button>
                </div>
              )}
            </div>

            <Separator />

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">Existing tokens</p>
                {apiKeysLoading && (
                  <Icons.Loader className="h-4 w-4 animate-spin text-muted-foreground" />
                )}
              </div>

              {!apiKeysLoading && (!apiKeys || apiKeys.length === 0) && (
                <p className="text-sm text-muted-foreground">
                  No tokens yet. Create a token above to get started.
                </p>
              )}

              {apiKeys && apiKeys.length > 0 && (
                <div className="space-y-2">
                  {apiKeys.map((key) => (
                    <div
                      key={key.id}
                      className="flex items-start justify-between gap-3 rounded-md border bg-card px-3 py-2"
                    >
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium">
                            {key.name}
                          </span>
                          <Badge variant="outline" className="text-[10px]">
                            {key.scope}
                          </Badge>
                          {key.expires_at && (
                            <Badge
                              variant="outline"
                              className="text-[10px] text-amber-700 dark:text-amber-400"
                            >
                              Expires{" "}
                              {new Date(key.expires_at).toLocaleDateString()}
                            </Badge>
                          )}
                        </div>
                        <p className="text-xs font-mono text-muted-foreground">
                          {key.prefix}••••••
                        </p>
                        <p className="text-xs text-muted-foreground">
                          Created{" "}
                          {new Date(key.created_at).toLocaleDateString()}
                          {key.last_used_at
                            ? ` · Last used ${new Date(
                                key.last_used_at,
                              ).toLocaleDateString()}`
                            : " · Never used"}
                        </p>
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="shrink-0"
                        disabled={revokeAPIKeyMutation.isPending}
                        onClick={() => revokeAPIKeyMutation.mutate(key.id)}
                      >
                        Revoke
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* GitHub Integration Card */}
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
            {isProjectConnected ? (
              <div className="rounded-md bg-green-500/10 p-4 space-y-2">
                <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
                  <Icons.Check className="h-4 w-4" />
                  <span className="font-medium">Connected to GitHub</span>
                </div>
                {project.repo_url && (
                  <a
                    href={project.repo_url}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
                  >
                    <Icons.Link2 className="h-3 w-3" />
                    {project.repo_url}
                  </a>
                )}
              </div>
            ) : (
              <>
                {loadingInstallations ? (
                  <div className="flex items-center gap-2 text-muted-foreground">
                    <Icons.Loader className="h-4 w-4 animate-spin" />
                    Loading installations...
                  </div>
                ) : installations.length === 0 ? (
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
                ) : (
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
                          {installations.map((inst) => (
                            <SelectItem key={inst.id} value={inst.id}>
                              {inst.account_login} ({inst.account_type})
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

                    {selectedInstallation && (
                      <div className="space-y-2">
                        <Label htmlFor="repo">Repository</Label>
                        {loadingRepos ? (
                          <div className="flex items-center gap-2 text-muted-foreground">
                            <Icons.Loader className="h-4 w-4 animate-spin" />
                            Loading repositories...
                          </div>
                        ) : (
                          <Select
                            value={selectedRepo}
                            onValueChange={setSelectedRepo}
                          >
                            <SelectTrigger id="repo" className="w-full">
                              <SelectValue placeholder="Select a repository" />
                            </SelectTrigger>
                            <SelectContent>
                              {repos.map((repo) => (
                                <SelectItem
                                  key={repo.id}
                                  value={repo.id.toString()}
                                >
                                  {repo.full_name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        )}
                      </div>
                    )}

                    {connectToGitHubMutation.error && (
                      <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                        {connectToGitHubMutation.error.message}
                      </div>
                    )}

                    {connectSuccess && (
                      <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
                        Project connected to GitHub successfully!
                      </div>
                    )}

                    <Button
                      onClick={handleConnectToGitHub}
                      disabled={
                        !selectedInstallation ||
                        !selectedRepo ||
                        connectToGitHubMutation.isPending
                      }
                    >
                      {connectToGitHubMutation.isPending ? (
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
                  </>
                )}
              </>
            )}
          </CardContent>
        </Card>

        {/* Danger Zone Card */}
        <Card className="border-destructive/50 py-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-destructive">
              <Icons.Warning className="h-5 w-5" />
              Danger Zone
            </CardTitle>
            <CardDescription>
              Irreversible and destructive actions.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-1">
                <p className="font-medium">Delete this project</p>
                <p className="text-sm text-muted-foreground">
                  Once you delete a project, there is no going back. All runs,
                  logs, and associated data will be permanently removed.
                </p>
              </div>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive">
                    <Icons.Trash className="mr-2 h-4 w-4" />
                    Delete Project
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      Are you absolutely sure?
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      This action cannot be undone. This will permanently delete
                      the <strong>{project.name}</strong> project and all of its
                      data including runs, logs, and configurations.
                    </AlertDialogDescription>
                  </AlertDialogHeader>

                  <Separator />

                  <div className="space-y-2">
                    <Label htmlFor="confirm-delete">
                      Type{" "}
                      <code className="bg-muted px-1 rounded">
                        {project.slug}
                      </code>{" "}
                      to confirm:
                    </Label>
                    <Input
                      id="confirm-delete"
                      value={deleteConfirmText}
                      onChange={(e) => setDeleteConfirmText(e.target.value)}
                      placeholder={project.slug}
                    />
                  </div>

                  {deleteProjectMutation.error && (
                    <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                      {deleteProjectMutation.error.message}
                    </div>
                  )}

                  <AlertDialogFooter>
                    <AlertDialogCancel onClick={() => setDeleteConfirmText("")}>
                      Cancel
                    </AlertDialogCancel>
                    <AlertDialogAction
                      variant="destructive"
                      onClick={handleDelete}
                      disabled={!canDelete || deleteProjectMutation.isPending}
                    >
                      {deleteProjectMutation.isPending ? (
                        <>
                          <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                          Deleting...
                        </>
                      ) : (
                        "Delete Project"
                      )}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
