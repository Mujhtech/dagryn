import { createFileRoute, Link } from "@tanstack/react-router";
import { useProjects } from "~/hooks/queries";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { CreateProjectDialog } from "~/components/projects/create-project-dialog";
import { Icons } from "~/components/icons";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute("/_dashboard_layout/projects/")({
  component: ProjectsPage,
  head: () => {
    return generateMetadata({ title: "Projects" });
  },
});

function ProjectsPage() {
  const {
    data: projectsData,
    isLoading: projectsLoading,
    error: projectsError,
  } = useProjects();

  const loading = projectsLoading;
  const projects = projectsData?.data ?? [];
  const error = projectsError?.message;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
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

  const CreateProjectButtons = (
    <div className="flex gap-2">
      <CreateProjectDialog />
      <Button variant="outline" asChild>
        <Link to="/projects/new/github">
          <Icons.Github className="mr-2 h-4 w-4" />
          Import from GitHub
        </Link>
      </Button>
    </div>
  );

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Projects</h1>
          <p className="text-muted-foreground">Manage your workflow projects</p>
        </div>
        {CreateProjectButtons}
      </div>

      {projects.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Icons.Folder className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold">No projects yet</h3>
            <p className="text-muted-foreground text-center mt-1 mb-4">
              Create your first project to get started with Dagryn
            </p>
            {CreateProjectButtons}
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
                    <Icons.Users className="mr-1 h-4 w-4" />
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
