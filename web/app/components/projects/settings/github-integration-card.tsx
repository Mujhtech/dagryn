import type { GitHubAppInstallation, GitHubRepo, Project } from "~/lib/api";
import { Button } from "~/components/ui/button";
import { Label } from "~/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Icons } from "~/components/icons";

type GitHubIntegrationCardProps = {
  project: Project;
  installations: GitHubAppInstallation[];
  selectedInstallation: string;
  setSelectedInstallation: (value: string) => void;
  repos: GitHubRepo[];
  selectedRepo: string;
  setSelectedRepo: (value: string) => void;
  loadingInstallations: boolean;
  loadingRepos: boolean;
  isConnected: boolean;
  connectError?: string;
  connectSuccess: boolean;
  connectPending: boolean;
  onConnect: () => void;
};

export function GitHubIntegrationCard({
  project,
  installations,
  selectedInstallation,
  setSelectedInstallation,
  repos,
  selectedRepo,
  setSelectedRepo,
  loadingInstallations,
  loadingRepos,
  isConnected,
  connectError,
  connectSuccess,
  connectPending,
  onConnect,
}: GitHubIntegrationCardProps) {
  return (
    <Card className="py-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Icons.Github className="h-5 w-5" />
          GitHub Integration
        </CardTitle>
        <CardDescription>
          Connect this project to a GitHub repository to enable webhook triggers.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {isConnected ? (
          <div className="rounded-md bg-green-500/10 p-4 space-y-2">
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
        ) : (
          <>
            {loadingInstallations ? (
              <div className="flex items-center gap-2 text-muted-foreground">
                <Icons.Loader className="h-4 w-4 animate-spin" />
                Loading installations...
              </div>
            ) : installations.length === 0 ? (
              <div className="text-sm text-muted-foreground">
                No GitHub App installations found. {" "}
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
                  <Select value={selectedInstallation} onValueChange={setSelectedInstallation}>
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
                  <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                    {connectError}
                  </div>
                ) : null}

                {connectSuccess ? (
                  <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
                    Project connected to GitHub successfully!
                  </div>
                ) : null}

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
              </>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
