import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "~/lib/auth";
import {
  useGitHubAppInstallations,
  useGitHubAppRepos,
  useProjects,
} from "~/hooks/queries";
import { useCreateProject } from "~/hooks/mutations";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { ScrollArea } from "~/components/ui/scroll-area";
import { FolderKanban, Plus, Users, Loader2, Github } from "lucide-react";
import type { GitHubRepo, GitHubAppInstallation } from "~/lib/api";

export const Route = createFileRoute("/projects/")({
  component: ProjectsPage,
});

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function ProjectsPage() {
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const githubAppInstallUrl =
    "https://github.com/apps/dagryn-dev/installations/new";

  // Use TanStack Query for data fetching
  const {
    data: projectsData,
    isLoading: projectsLoading,
    error: projectsError,
  } = useProjects();

  // Use mutation for creating projects
  const createProjectMutation = useCreateProject();

  // Create project modal state
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createSlug, setCreateSlug] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createVisibility, setCreateVisibility] = useState<
    "private" | "public"
  >("private");
  const [slugEdited, setSlugEdited] = useState(false);

  // Import from GitHub
  const [isImportOpen, setIsImportOpen] = useState(false);
  const [selectedInstallation, setSelectedInstallation] =
    useState<GitHubAppInstallation | null>(null);
  const [selectedGitHubRepo, setSelectedGitHubRepo] =
    useState<GitHubRepo | null>(null);
  const [importName, setImportName] = useState("");
  const [importSlug, setImportSlug] = useState("");
  const [importSlugEdited, setImportSlugEdited] = useState(false);
  const [repoSearch, setRepoSearch] = useState("");

  // Legacy OAuth-based GitHub repos (fallback)
  const {
    data: githubRepos = [],
    isLoading: githubReposLoading,
    error: githubReposError,
  } = useQuery({
    queryKey: queryKeys.githubRepos,
    queryFn: async () => {
      const response = await api.listGitHubRepos();
      return response.data;
    },
    enabled: isImportOpen,
    retry: false,
  });

  // GitHub App installations and repos
  const {
    data: installations = [],
    isLoading: installationsLoading,
    error: installationsError,
  } = useGitHubAppInstallations();

  const {
    data: appRepos = [],
    isLoading: appReposLoading,
    error: appReposError,
  } = useGitHubAppRepos(selectedInstallation ? selectedInstallation.id : null);

  useEffect(() => {
    if (selectedGitHubRepo && !importSlugEdited) {
      setImportName(selectedGitHubRepo.full_name);
      setImportSlug(slugify(selectedGitHubRepo.full_name));
    }
  }, [selectedGitHubRepo, importSlugEdited]);

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, authLoading, navigate]);

  // Auto-generate slug from name
  useEffect(() => {
    if (!slugEdited && createName) {
      setCreateSlug(slugify(createName));
    }
  }, [createName, slugEdited]);

  const handleCreateProject = async () => {
    if (!createName.trim()) {
      return;
    }
    if (!createSlug.trim()) {
      return;
    }

    createProjectMutation.mutate(
      {
        name: createName.trim(),
        slug: createSlug.trim(),
        description: createDescription.trim() || undefined,
        visibility: createVisibility,
      },
      {
        onSuccess: (project) => {
          resetCreateForm();
          setIsCreateOpen(false);
          // Navigate to the new project
          navigate({
            to: "/projects/$projectId",
            params: { projectId: project.id },
          });
        },
      }
    );
  };

  const resetCreateForm = () => {
    setCreateName("");
    setCreateSlug("");
    setCreateDescription("");
    setCreateVisibility("private");
    setSlugEdited(false);
    createProjectMutation.reset();
  };

  const handleOpenChange = (open: boolean) => {
    setIsCreateOpen(open);
    if (!open) {
      resetCreateForm();
    }
  };

  const handleImportOpenChange = (open: boolean) => {
    setIsImportOpen(open);
    if (!open) {
      setSelectedInstallation(null);
      setSelectedGitHubRepo(null);
      setImportName("");
      setImportSlug("");
      setImportSlugEdited(false);
      setRepoSearch("");
    }
  };

  const effectiveRepos =
    selectedInstallation && appRepos.length > 0 ? appRepos : githubRepos;

  const filteredGitHubRepos = repoSearch.trim()
    ? effectiveRepos.filter((r) =>
        r.full_name.toLowerCase().includes(repoSearch.trim().toLowerCase())
      )
    : effectiveRepos;

  const handleCreateFromGitHub = () => {
    if (!selectedGitHubRepo || !importName.trim() || !importSlug.trim()) return;
    createProjectMutation.mutate(
      {
        name: importName.trim(),
        slug: importSlug.trim(),
        repo_url: selectedGitHubRepo.clone_url,
        github_installation_id: selectedInstallation?.id ?? "",
        github_repo_id: selectedGitHubRepo.id,
        visibility: "private",
      },
      {
        onSuccess: (project) => {
          handleImportOpenChange(false);
          navigate({
            to: "/projects/$projectId",
            params: { projectId: project.id },
          });
        },
      }
    );
  };

  const loading = authLoading || projectsLoading;
  const projects = projectsData?.data ?? [];
  const error = projectsError?.message;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>{error}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const needsGitHubLogin =
    githubReposError &&
    "status" in githubReposError &&
    (githubReposError as { status?: number }).status === 403;

  const CreateProjectButton = (
    <div className="flex gap-2">
      <Dialog open={isCreateOpen} onOpenChange={handleOpenChange}>
        <DialogTrigger asChild>
          <Button>
            <Plus className="mr-2 h-4 w-4" />
            New Project
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>Create Project</DialogTitle>
            <DialogDescription>
              Create a new workflow project. You can configure workflows after
              creation.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={createName}
                onChange={(e) => setCreateName(e.target.value)}
                placeholder="My Project"
                disabled={createProjectMutation.isPending}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="slug">Slug</Label>
              <Input
                id="slug"
                value={createSlug}
                onChange={(e) => {
                  setCreateSlug(e.target.value);
                  setSlugEdited(true);
                }}
                placeholder="my-project"
                disabled={createProjectMutation.isPending}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                URL-friendly identifier for your project
              </p>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="description">Description (optional)</Label>
              <Input
                id="description"
                value={createDescription}
                onChange={(e) => setCreateDescription(e.target.value)}
                placeholder="A brief description of your project"
                disabled={createProjectMutation.isPending}
              />
            </div>
            <div className="grid gap-2">
              <Label>Visibility</Label>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant={
                    createVisibility === "private" ? "default" : "outline"
                  }
                  size="sm"
                  onClick={() => setCreateVisibility("private")}
                  disabled={createProjectMutation.isPending}
                >
                  Private
                </Button>
                <Button
                  type="button"
                  variant={
                    createVisibility === "public" ? "default" : "outline"
                  }
                  size="sm"
                  onClick={() => setCreateVisibility("public")}
                  disabled={createProjectMutation.isPending}
                >
                  Public
                </Button>
              </div>
            </div>
            {createProjectMutation.error && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {createProjectMutation.error.message}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={createProjectMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              onClick={handleCreateProject}
              disabled={createProjectMutation.isPending}
            >
              {createProjectMutation.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating...
                </>
              ) : (
                "Create Project"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={isImportOpen} onOpenChange={handleImportOpenChange}>
        <DialogTrigger asChild>
          <Button variant="outline">
            <Github className="mr-2 h-4 w-4" />
            Import from GitHub
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>Import from GitHub</DialogTitle>
            <DialogDescription>
              Select a repository to create a project. Prefer the GitHub App
              installation flow when available; legacy OAuth is used as a
              fallback.
            </DialogDescription>
          </DialogHeader>
          {githubReposLoading || installationsLoading || appReposLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : needsGitHubLogin ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4 text-sm">
              <p className="mb-2">
                No GitHub account linked. Log in with GitHub to import
                repositories.
              </p>
              <Button asChild variant="outline" size="sm">
                <Link to="/login">Go to login</Link>
              </Button>
            </div>
          ) : installationsError && !needsGitHubLogin ? (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive mb-3">
              Failed to load GitHub App installations.
            </div>
          ) : selectedGitHubRepo ? (
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label>Repository</Label>
                <p className="text-sm text-muted-foreground font-mono">
                  {selectedGitHubRepo.full_name}
                </p>
              </div>
              <div className="grid gap-2">
                <Label>Repo URL (stored for runs and webhooks)</Label>
                <p className="text-xs text-muted-foreground font-mono break-all bg-muted/50 rounded px-2 py-1.5">
                  {selectedGitHubRepo.clone_url}
                </p>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="import-name">Project name</Label>
                <Input
                  id="import-name"
                  value={importName}
                  onChange={(e) => setImportName(e.target.value)}
                  placeholder="My Project"
                  disabled={createProjectMutation.isPending}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="import-slug">Slug</Label>
                <Input
                  id="import-slug"
                  value={importSlug}
                  onChange={(e) => {
                    setImportSlug(e.target.value);
                    setImportSlugEdited(true);
                  }}
                  placeholder="my-project"
                  disabled={createProjectMutation.isPending}
                  className="font-mono"
                />
              </div>
              {createProjectMutation.error && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  {createProjectMutation.error.message}
                </div>
              )}
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setSelectedGitHubRepo(null)}
                  disabled={createProjectMutation.isPending}
                >
                  Back
                </Button>
                <Button
                  onClick={handleCreateFromGitHub}
                  disabled={createProjectMutation.isPending}
                >
                  {createProjectMutation.isPending ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Creating...
                    </>
                  ) : (
                    "Create Project"
                  )}
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <>
              {githubReposError && !needsGitHubLogin && !installationsError && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive mb-3">
                  {githubReposError instanceof Error
                    ? githubReposError.message
                    : "Failed to load repositories."}
                </div>
              )}
              {installations.length === 0 ? (
                <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4 text-sm mb-3">
                  <p className="mb-2">
                    No GitHub App installations found. Install the Dagryn GitHub
                    App on your GitHub account or organization to use the
                    app-based flow. For now, you can still import using your
                    linked GitHub account.
                  </p>
                  {githubAppInstallUrl && (
                    <Button asChild size="sm" variant="outline">
                      <a
                        href={githubAppInstallUrl}
                        target="_blank"
                        rel="noreferrer"
                      >
                        Open GitHub App install page
                      </a>
                    </Button>
                  )}
                </div>
              ) : (
                <div className="mb-3 space-y-2">
                  <Label>GitHub App installation</Label>
                  <div className="flex flex-wrap gap-2">
                    {installations.map((inst) => (
                      <Button
                        key={inst.id}
                        type="button"
                        size="sm"
                        variant={
                          selectedInstallation?.id === inst.id
                            ? "default"
                            : "outline"
                        }
                        onClick={() =>
                          setSelectedInstallation(
                            selectedInstallation?.id === inst.id ? null : inst
                          )
                        }
                      >
                        {inst.account_login} ({inst.account_type})
                      </Button>
                    ))}
                  </div>
                </div>
              )}
              <div className="grid gap-2 mb-2">
                <Label htmlFor="repo-search">Search repos</Label>
                <Input
                  id="repo-search"
                  placeholder="Filter by name..."
                  value={repoSearch}
                  onChange={(e) => setRepoSearch(e.target.value)}
                  className="font-mono"
                />
              </div>
              <ScrollArea className="h-[300px] rounded-md border">
                <div className="p-2 space-y-1">
                  {filteredGitHubRepos.length === 0 && !githubReposError && (
                    <p className="py-4 text-center text-sm text-muted-foreground">
                      {githubRepos.length === 0
                        ? "No repositories found."
                        : "No repos match your search."}
                    </p>
                  )}
                  {filteredGitHubRepos.map((repo) => (
                    <button
                      key={repo.id}
                      type="button"
                      className="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm hover:bg-muted"
                      onClick={() => setSelectedGitHubRepo(repo)}
                    >
                      <span className="font-mono truncate">
                        {repo.full_name}
                      </span>
                      {repo.private && (
                        <Badge variant="secondary" className="ml-2 shrink-0">
                          Private
                        </Badge>
                      )}
                    </button>
                  ))}
                </div>
              </ScrollArea>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Projects</h1>
          <p className="text-muted-foreground">Manage your workflow projects</p>
        </div>
        {CreateProjectButton}
      </div>

      {projects.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <FolderKanban className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold">No projects yet</h3>
            <p className="text-muted-foreground text-center mt-1 mb-4">
              Create your first project to get started with Dagryn
            </p>
            {CreateProjectButton}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {projects.map((project) => (
            <Link
              key={project.id}
              to="/projects/$projectId"
              params={{ projectId: project.id }}
              className="block"
            >
              <Card className="hover:border-primary/50 transition-colors cursor-pointer h-full py-3">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="space-y-1">
                      <CardTitle className="text-lg">{project.name}</CardTitle>
                      <p className="text-sm text-muted-foreground font-mono">
                        {project.slug}
                      </p>
                    </div>
                    <Badge
                      variant={
                        project.visibility === "public"
                          ? "default"
                          : "secondary"
                      }
                    >
                      {project.visibility}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  {project.description && (
                    <p className="text-sm text-muted-foreground line-clamp-2 mb-4">
                      {project.description}
                    </p>
                  )}
                  <div className="flex items-center text-sm text-muted-foreground">
                    <Users className="mr-1 h-4 w-4" />
                    {project.member_count} member
                    {project.member_count !== 1 ? "s" : ""}
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
