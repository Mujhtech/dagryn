import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useProjects } from "~/hooks/queries";
import { useCreateProject } from "~/hooks/mutations";
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
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/projects/")({
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

  const {
    data: projectsData,
    isLoading: projectsLoading,
    error: projectsError,
  } = useProjects();

  const createProjectMutation = useCreateProject();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createSlug, setCreateSlug] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createVisibility, setCreateVisibility] = useState<
    "private" | "public"
  >("private");
  const [slugEdited, setSlugEdited] = useState(false);

  useEffect(() => {
    if (!slugEdited && createName) {
      setCreateSlug(slugify(createName));
    }
  }, [createName, slugEdited]);

  const handleCreateProject = () => {
    if (!createName.trim() || !createSlug.trim()) {
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
          navigate({
            to: "/projects/$projectId",
            params: { projectId: project.id },
          });
        },
      },
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

  const CreateProjectButton = (
    <div className="flex gap-2">
      <Dialog open={isCreateOpen} onOpenChange={handleOpenChange}>
        <DialogTrigger asChild>
          <Button>
            <Icons.Plus className="mr-2 h-4 w-4" />
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
                  <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                  Creating...
                </>
              ) : (
                "Create Project"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
        {CreateProjectButton}
      </div>

      {projects.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Icons.Folder className="h-12 w-12 text-muted-foreground mb-4" />
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
