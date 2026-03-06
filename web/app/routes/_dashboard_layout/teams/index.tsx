import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTeams } from "~/hooks/queries";
import { useCreateTeam } from "~/hooks/mutations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
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
import { generateMetadata } from "~/lib/metadata";

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export const Route = createFileRoute("/_dashboard_layout/teams/")({
  component: TeamsPage,
  head: () => {
    return generateMetadata({ title: "Teams" });
  },
});

function TeamsPage() {
  const navigate = useNavigate();

  const {
    data: teamsData,
    isLoading: teamsLoading,
    error: teamsError,
  } = useTeams();
  const createTeamMutation = useCreateTeam();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createSlug, setCreateSlug] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [slugEdited, setSlugEdited] = useState(false);

  useEffect(() => {
    if (!slugEdited && createName) {
      setCreateSlug(slugify(createName));
    }
  }, [createName, slugEdited]);

  const handleCreateTeam = async () => {
    if (!createName.trim()) return;
    createTeamMutation.mutate(
      {
        name: createName.trim(),
        slug: createSlug.trim() || undefined,
        description: createDescription.trim() || undefined,
      },
      {
        onSuccess: (team) => {
          setCreateName("");
          setCreateSlug("");
          setCreateDescription("");
          setSlugEdited(false);
          setIsCreateOpen(false);
          navigate({ to: "/teams/$teamId", params: { teamId: team.id } });
        },
      },
    );
  };

  const loading = teamsLoading;
  const teams =
    (
      teamsData as
        | {
            data?: Array<{
              id: string;
              name: string;
              slug: string;
              description?: string;
              member_count: number;
            }>;
          }
        | undefined
    )?.data ?? [];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (teamsError) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>{(teamsError as Error).message}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Teams</h1>
          <p className="text-muted-foreground">Manage your teams and members</p>
        </div>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Icons.Plus className="mr-2 h-4 w-4" />
              New Team
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[425px]">
            <DialogHeader>
              <DialogTitle>Create Team</DialogTitle>
              <DialogDescription>
                Create a new team and invite members.
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="team-name">Name</Label>
                <Input
                  id="team-name"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="Engineering"
                  disabled={createTeamMutation.isPending}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="team-slug">Slug</Label>
                <Input
                  id="team-slug"
                  value={createSlug}
                  onChange={(e) => {
                    setCreateSlug(e.target.value);
                    setSlugEdited(true);
                  }}
                  placeholder="engineering"
                  className="font-mono"
                  disabled={createTeamMutation.isPending}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="team-desc">Description (optional)</Label>
                <Input
                  id="team-desc"
                  value={createDescription}
                  onChange={(e) => setCreateDescription(e.target.value)}
                  placeholder="Engineering team"
                  disabled={createTeamMutation.isPending}
                />
              </div>
              {createTeamMutation.error && (
                <p className="text-sm text-destructive">
                  {createTeamMutation.error.message}
                </p>
              )}
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setIsCreateOpen(false)}
                disabled={createTeamMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                onClick={handleCreateTeam}
                disabled={createTeamMutation.isPending || !createName.trim()}
              >
                {createTeamMutation.isPending ? (
                  <>
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  "Create"
                )}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {teams.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Icons.Users className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold">No teams yet</h3>
            <p className="text-muted-foreground text-center mt-1 mb-4">
              Create a team to collaborate and manage projects together
            </p>
            <Button onClick={() => setIsCreateOpen(true)}>
              <Icons.Plus className="mr-2 h-4 w-4" />
              New Team
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {teams.map(
            (team: {
              id: string;
              name: string;
              slug: string;
              description?: string;
              member_count: number;
            }) => (
              <Link
                key={team.id}
                to="/teams/$teamId"
                params={{ teamId: team.id }}
                className="block"
              >
                <Card className="hover:border-primary/50 py-3 transition-colors cursor-pointer h-full">
                  <CardHeader className="pb-2">
                    <CardTitle className="text-lg">{team.name}</CardTitle>
                    <p className="text-sm text-muted-foreground font-mono">
                      {team.slug}
                    </p>
                  </CardHeader>
                  <CardContent>
                    {team.description && (
                      <p className="text-sm text-muted-foreground line-clamp-2 mb-4">
                        {team.description}
                      </p>
                    )}
                    <div className="flex items-center text-sm text-muted-foreground">
                      <Icons.Users className="mr-1 h-4 w-4" />
                      {team.member_count} member
                      {team.member_count !== 1 ? "s" : ""}
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ),
          )}
        </div>
      )}
    </div>
  );
}
