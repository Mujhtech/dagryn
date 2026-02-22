import { createFileRoute, Link, Outlet, useLocation } from "@tanstack/react-router";
import { useProject } from "~/hooks/queries";
import { Button } from "~/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Icons } from "~/components/icons";
import { cn } from "~/lib/utils";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/settings",
)({
  component: ProjectSettingsLayout,
  head: () => {
    return generateMetadata({ title: "Project Settings" });
  },
});

const navItems = [
  { label: "General", to: ".", icon: Icons.Settings },
  { label: "Git & Repository", to: "./git", icon: Icons.GitBranch },
  { label: "API Keys", to: "./api-keys", icon: Icons.Key },
] as const;

function ProjectSettingsLayout() {
  const { projectId } = Route.useParams();
  const location = useLocation();

  const {
    data: project,
    isLoading: projectLoading,
    error: projectError,
  } = useProject(projectId);

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

  const settingsBase = `/projects/${projectId}/settings`;
  const currentPath = location.pathname.replace(/\/$/, "");

  return (
    <div className="space-y-6 px-6 @container/main py-3">
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

      <div className="flex flex-col gap-6 md:flex-row">
        <nav className="flex flex-row gap-1 md:w-48 md:shrink-0 md:flex-col">
          {navItems.map((item) => {
            const itemPath = item.to === "."
              ? settingsBase
              : `${settingsBase}/${item.to.replace("./", "")}`;
            const isActive = currentPath === itemPath;

            return (
              <Link
                key={item.to}
                to={item.to}
                from={`/projects/$projectId/settings`}
                params={{ projectId }}
                className={cn(
                  "flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="flex-1 max-w-2xl">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
