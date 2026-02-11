import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  useGitHubAppInstallations,
  useGitHubAppRepos,
  useGitHubWorkflowTranslation,
} from "~/hooks/queries";
import { useCreateProject } from "~/hooks/mutations";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { ScrollArea } from "~/components/ui/scroll-area";
import { Textarea } from "~/components/ui/textarea";
import { Icons } from "~/components/icons";
import type { GitHubAppInstallation, GitHubRepo } from "~/lib/api";
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "~/components/ui/command";

export const Route = createFileRoute("/_dashboard_layout/projects/new/github")({
  component: ImportFromGitHubPage,
});

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function buildDagrynConfigFromSnippet(snippet: string): string {
  const trimmed = snippet.trim();
  if (trimmed.startsWith("[workflow]") || trimmed.includes("\n[workflow]")) {
    return `${trimmed}\n`;
  }
  return [
    "[workflow]",
    'name = "default"',
    "default = true",
    "",
    trimmed,
    "",
  ].join("\n");
}

function ImportFromGitHubPage() {
  const navigate = useNavigate();
  const createProjectMutation = useCreateProject();
  const githubAppInstallUrl =
    "https://github.com/apps/dagryn-dev/installations/new";

  const [selectedInstallation, setSelectedInstallation] =
    useState<GitHubAppInstallation | null>(null);
  const [selectedGitHubRepo, setSelectedGitHubRepo] =
    useState<GitHubRepo | null>(null);
  const [importName, setImportName] = useState("");
  const [importSlug, setImportSlug] = useState("");
  const [importSlugEdited, setImportSlugEdited] = useState(false);
  const [repoSearch, setRepoSearch] = useState("");
  const [useDetectedWorkflow, setUseDetectedWorkflow] = useState(true);
  const [workflowDraft, setWorkflowDraft] = useState("");
  const [workflowSyncError, setWorkflowSyncError] = useState("");
  const [workflowShowFull, setWorkflowShowFull] = useState(false);
  const [pendingProjectId, setPendingProjectId] = useState<string | null>(null);

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
    retry: false,
  });

  const {
    data: installations = [],
    isLoading: installationsLoading,
    error: installationsError,
  } = useGitHubAppInstallations();

  const { data: appRepos = [], isLoading: appReposLoading } = useGitHubAppRepos(
    selectedInstallation ? selectedInstallation.id : null,
  );

  const {
    data: workflowTranslation,
    isLoading: workflowTranslationLoading,
    error: workflowTranslationError,
  } = useGitHubWorkflowTranslation(
    selectedGitHubRepo ? selectedGitHubRepo.full_name : null,
    selectedInstallation?.id ?? null,
  );

  useEffect(() => {
    if (!selectedGitHubRepo) {
      setWorkflowDraft("");
      setUseDetectedWorkflow(false);
      setWorkflowSyncError("");
      setWorkflowShowFull(false);
      setPendingProjectId(null);
      return;
    }

    if (workflowTranslation?.detected) {
      setWorkflowDraft(workflowTranslation.tasks_toml.trim());
      setUseDetectedWorkflow(true);
    } else {
      setWorkflowDraft("");
      setUseDetectedWorkflow(false);
    }

    setWorkflowSyncError("");
    setWorkflowShowFull(false);
    setPendingProjectId(null);
  }, [selectedGitHubRepo, workflowTranslation]);

  useEffect(() => {
    if (selectedGitHubRepo && !importSlugEdited) {
      setImportName(selectedGitHubRepo.full_name);
      setImportSlug(slugify(selectedGitHubRepo.full_name));
    }
  }, [selectedGitHubRepo, importSlugEdited]);

  const effectiveRepos =
    selectedInstallation && appRepos.length > 0 ? appRepos : githubRepos;

  const filteredGitHubRepos = repoSearch.trim()
    ? effectiveRepos.filter((repo) =>
        repo.full_name.toLowerCase().includes(repoSearch.trim().toLowerCase()),
      )
    : effectiveRepos;

  const handleCreateFromGitHub = async () => {
    if (!selectedGitHubRepo || !importName.trim() || !importSlug.trim()) {
      return;
    }

    setWorkflowSyncError("");

    try {
      const project = await createProjectMutation.mutateAsync({
        name: importName.trim(),
        slug: importSlug.trim(),
        repo_url: selectedGitHubRepo.clone_url,
        github_installation_id: selectedInstallation?.id ?? "",
        github_repo_id: selectedGitHubRepo.id,
        visibility: "private",
      });

      let syncError = "";
      if (useDetectedWorkflow && workflowDraft.trim()) {
        const rawConfig = buildDagrynConfigFromSnippet(workflowDraft);
        try {
          await api.syncProjectWorkflowFromToml(project.id, rawConfig);
        } catch (err) {
          syncError =
            err instanceof Error
              ? err.message
              : "Failed to sync workflow from detected configuration.";
        }
      }

      if (syncError) {
        setWorkflowSyncError(syncError);
        setPendingProjectId(project.id);
        return;
      }

      navigate({
        to: "/projects/$projectId",
        params: { projectId: project.id },
      });
    } catch (err) {
      setWorkflowSyncError(
        err instanceof Error ? err.message : "Failed to create project.",
      );
    }
  };

  const loading = githubReposLoading || installationsLoading || appReposLoading;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const needsGitHubLogin =
    githubReposError &&
    "status" in githubReposError &&
    (githubReposError as { status?: number }).status === 403;

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            Import from GitHub
          </h1>
          <p className="text-muted-foreground">
            Create a project directly from a GitHub repository
          </p>
        </div>
        <Button variant="outline" asChild>
          <Link to="/projects">
            <Icons.ArrowLeft className="mr-2 h-4 w-4" />
            Back to Projects
          </Link>
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Repository Selection</CardTitle>
          <CardDescription>
            Select a repository to create a project. GitHub App installation is
            preferred; OAuth is used as fallback.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {needsGitHubLogin ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4 text-sm">
              <p className="mb-2">
                No GitHub account linked. Log in with GitHub to import
                repositories.
              </p>
              <Button asChild variant="outline" size="sm">
                <Link to="/login">Go to login</Link>
              </Button>
            </div>
          ) : selectedGitHubRepo ? (
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <div className="flex flex-col gap-4">
                <div className="grid gap-2">
                  <Label>Repository</Label>
                  <p className="text-sm text-muted-foreground font-mono">
                    {selectedGitHubRepo.full_name}
                  </p>
                </div>

                <div className="grid gap-2">
                  <Label>Repo URL (stored for runs and webhooks)</Label>
                  <Input value={selectedGitHubRepo.clone_url} readOnly />
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
              </div>

              <div className="grid gap-4">
                <div className="grid gap-2">
                  <Label>GitHub workflow detection</Label>
                  {workflowTranslationLoading ? (
                    <p className="text-sm text-muted-foreground">
                      Checking .github/workflows...
                    </p>
                  ) : workflowTranslationError ? (
                    <p className="text-sm text-destructive">
                      Failed to inspect workflows.
                    </p>
                  ) : workflowTranslation?.detected ? (
                    <div className="rounded-md border bg-muted/40 p-3 space-y-3">
                      <div className="flex items-center gap-2">
                        <Checkbox
                          id="use-detected-workflow"
                          checked={useDetectedWorkflow}
                          onCheckedChange={(checked) =>
                            setUseDetectedWorkflow(Boolean(checked))
                          }
                        />
                        <Label
                          htmlFor="use-detected-workflow"
                          className="text-sm"
                        >
                          Use detected workflow (auto-sync after create)
                        </Label>
                      </div>

                      <p className="text-xs text-muted-foreground">
                        Found {workflowTranslation.workflows.length} workflow
                        {workflowTranslation.workflows.length === 1 ? "" : "s"}.
                        You can edit the generated tasks before creating the
                        project.
                      </p>

                      {useDetectedWorkflow && (
                        <>
                          <Textarea
                            value={workflowDraft}
                            onChange={(e) => setWorkflowDraft(e.target.value)}
                            className="min-h-[220px] font-mono text-xs"
                          />
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => setWorkflowShowFull((prev) => !prev)}
                            className="w-fit"
                          >
                            {workflowShowFull
                              ? "Hide full dagryn.toml"
                              : "Preview full dagryn.toml"}
                          </Button>
                          {workflowShowFull && (
                            <Textarea
                              value={buildDagrynConfigFromSnippet(
                                workflowDraft,
                              )}
                              readOnly
                              className="min-h-[220px] font-mono text-xs"
                            />
                          )}
                        </>
                      )}
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      No workflow configuration found in the selected
                      repository.
                    </p>
                  )}
                </div>

                {workflowSyncError && (
                  <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                    <p>{workflowSyncError}</p>
                    {pendingProjectId && (
                      <div className="mt-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() =>
                            navigate({
                              to: "/projects/$projectId",
                              params: { projectId: pendingProjectId },
                            })
                          }
                        >
                          Continue anyway
                        </Button>
                      </div>
                    )}
                  </div>
                )}

                {createProjectMutation.error && (
                  <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                    {createProjectMutation.error.message}
                  </div>
                )}
              </div>

              <div className="lg:col-span-2 flex items-center justify-end gap-2">
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
                      <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                      Creating...
                    </>
                  ) : (
                    "Create Project"
                  )}
                </Button>
              </div>
            </div>
          ) : (
            <>
              {githubReposError && !needsGitHubLogin && !installationsError && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  {githubReposError instanceof Error
                    ? githubReposError.message
                    : "Failed to load repositories."}
                </div>
              )}

              {installationsError && !needsGitHubLogin && (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  Failed to load GitHub App installations.
                </div>
              )}

              {installations.length === 0 ? (
                <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4 text-sm">
                  <p className="mb-2">
                    No GitHub App installations found. Install the Dagryn GitHub
                    App on your GitHub account or organization to use the
                    app-based flow. You can still import using your linked
                    GitHub account.
                  </p>
                  <Button asChild size="sm" variant="outline">
                    <a
                      href={githubAppInstallUrl}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open GitHub App install page
                    </a>
                  </Button>
                </div>
              ) : (
                <div className="space-y-2">
                  <Label>GitHub</Label>
                  <div className="flex flex-wrap gap-2">
                    <AccountSelection
                      installations={installations}
                      selectedInstallation={selectedInstallation}
                      onClick={(installation) => {
                        setSelectedInstallation(installation);
                      }}
                    />
                  </div>
                </div>
              )}

              <div className="grid gap-2">
                <Label htmlFor="repo-search">Search repos</Label>
                <Input
                  id="repo-search"
                  placeholder="Filter by name..."
                  value={repoSearch}
                  onChange={(e) => setRepoSearch(e.target.value)}
                  className="font-mono"
                />
              </div>

              <ScrollArea className="h-[420px] rounded-md border">
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
        </CardContent>
      </Card>
    </div>
  );
}

const AccountSelection = ({
  installations,
  selectedInstallation,
  onClick,
}: {
  installations: GitHubAppInstallation[];
  selectedInstallation: GitHubAppInstallation | null;
  onClick: (installation: GitHubAppInstallation | null) => void;
}) => {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="outline" className="w-fit" onClick={() => setOpen(true)}>
        {selectedInstallation ? (
          <span className="font-mono">
            {selectedInstallation.account_login} (
            {selectedInstallation.account_type})
          </span>
        ) : (
          "Select installation"
        )}
      </Button>
      <CommandDialog open={open} onOpenChange={setOpen}>
        <Command>
          <CommandInput placeholder="Type a command or search..." />
          <CommandList>
            <CommandEmpty>No results found.</CommandEmpty>
            <CommandGroup heading="Installations">
              {installations.map((installation) => (
                <CommandItem
                  key={installation.id}
                  value={installation.account_login}
                  onSelect={() => {
                    onClick(installation);
                  }}
                >
                  <span className="font-mono">
                    {installation.account_login} ({installation.account_type})
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup heading="Accounts">
              <CommandItem onSelect={() => onClick(null)}>
                <span className="font-mono">Linked GitHub account</span>
              </CommandItem>
            </CommandGroup>
            <CommandGroup heading="Settings">
              <CommandItem>
                <span className="font-mono">Connect GitHub account</span>
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </CommandDialog>
    </>
  );
};
