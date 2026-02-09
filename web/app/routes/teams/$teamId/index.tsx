import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useAuth } from "~/lib/auth";
import { useTeam, useTeamMembers, useTeamInvitations } from "~/hooks/queries";
import {
  useUpdateTeam,
  useDeleteTeam,
  useCreateTeamInvitation,
  useRevokeTeamInvitation,
  useRemoveTeamMember,
} from "~/hooks/mutations";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import type { Team, TeamMember, Invitation } from "~/lib/api";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/teams/$teamId/")({
  component: TeamDetailPage,
});

function TeamDetailPage() {
  const { teamId } = Route.useParams();
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  const {
    data: team,
    isLoading: teamLoading,
    error: teamError,
  } = useTeam(teamId);
  const { data: members, isLoading: membersLoading } = useTeamMembers(teamId);
  const { data: invitations, isLoading: invitationsLoading } =
    useTeamInvitations(teamId);

  const updateTeamMutation = useUpdateTeam(teamId);
  const deleteTeamMutation = useDeleteTeam();
  const createInvitationMutation = useCreateTeamInvitation(teamId);
  const revokeInvitationMutation = useRevokeTeamInvitation(teamId);
  const removeMemberMutation = useRemoveTeamMember(teamId);

  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [isInviteOpen, setIsInviteOpen] = useState(false);
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [isEditOpen, setIsEditOpen] = useState(false);

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, authLoading, navigate]);

  useEffect(() => {
    if (team) {
      setEditName(team.name);
      setEditDescription(team.description ?? "");
    }
  }, [team]);

  const handleUpdateTeam = () => {
    updateTeamMutation.mutate(
      { name: editName, description: editDescription || undefined },
      { onSuccess: () => setIsEditOpen(false) },
    );
  };

  const handleDeleteTeam = () => {
    if (
      !confirm(
        "Are you sure you want to delete this team? This cannot be undone.",
      )
    )
      return;
    deleteTeamMutation.mutate(teamId, {
      onSuccess: () => navigate({ to: "/teams" }),
    });
  };

  const handleCreateInvitation = () => {
    if (!inviteEmail.trim()) return;
    createInvitationMutation.mutate(
      { email: inviteEmail.trim(), role: inviteRole },
      {
        onSuccess: () => {
          setInviteEmail("");
          setInviteRole("member");
          setIsInviteOpen(false);
        },
      },
    );
  };

  const loading = authLoading || teamLoading;
  const membersList = (members as TeamMember[] | undefined) ?? [];
  const invitationsList = (invitations as Invitation[] | undefined) ?? [];

  if (loading && !team) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (teamError || !team) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>
            {(teamError as Error)?.message ?? "Team not found"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" asChild>
            <Link to="/teams">Back to Teams</Link>
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-3xl font-bold tracking-tight">
              {(team as Team).name}
            </h1>
            <Badge variant="secondary" className="font-mono">
              {(team as Team).slug}
            </Badge>
          </div>
          {(team as Team).description && (
            <p className="text-muted-foreground mt-1">
              {(team as Team).description}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
            <DialogTrigger asChild>
              <Button variant="outline">Edit</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Edit Team</DialogTitle>
                <DialogDescription>
                  Update team name and description.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label>Name</Label>
                  <Input
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    disabled={updateTeamMutation.isPending}
                  />
                </div>
                <div className="grid gap-2">
                  <Label>Description</Label>
                  <Input
                    value={editDescription}
                    onChange={(e) => setEditDescription(e.target.value)}
                    disabled={updateTeamMutation.isPending}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsEditOpen(false)}>
                  Cancel
                </Button>
                <Button
                  onClick={handleUpdateTeam}
                  disabled={updateTeamMutation.isPending}
                >
                  Save
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
          <Button
            variant="destructive"
            onClick={handleDeleteTeam}
            disabled={deleteTeamMutation.isPending}
          >
            Delete
          </Button>
        </div>
      </div>

      <Tabs defaultValue="members">
        <TabsList>
          <TabsTrigger value="members">
            <Icons.Users className="mr-2 h-4 w-4" />
            Members ({membersList.length})
          </TabsTrigger>
          <TabsTrigger value="invitations">
            <Icons.Mail className="mr-2 h-4 w-4" />
            Invitations (
            {invitationsList.filter((i) => i.status === "pending").length})
          </TabsTrigger>
        </TabsList>
        <TabsContent value="members" className="mt-4">
          <Card className="py-3">
            <CardHeader>
              <CardTitle>Members</CardTitle>
              <CardDescription>People in this team</CardDescription>
            </CardHeader>
            <CardContent>
              {membersLoading ? (
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              ) : membersList.length === 0 ? (
                <p className="text-muted-foreground">No members yet.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>User</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead>Joined</TableHead>
                      <TableHead className="w-[80px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {membersList.map((m) => (
                      <TableRow key={m.user.id}>
                        <TableCell>
                          <div className="font-medium">
                            {m.user.name || m.user.email}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {m.user.email}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{m.role}</Badge>
                        </TableCell>
                        <TableCell className="text-muted-foreground text-sm">
                          {new Date(m.joined_at).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => {
                              if (confirm("Remove this member?")) {
                                removeMemberMutation.mutate(m.user.id);
                              }
                            }}
                            disabled={removeMemberMutation.isPending}
                          >
                            <Icons.UserMinus className="h-4 w-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="invitations" className="mt-4">
          <Card className="py-3">
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Pending Invitations</CardTitle>
                <CardDescription>Invite people by email</CardDescription>
              </div>
              <Dialog open={isInviteOpen} onOpenChange={setIsInviteOpen}>
                <DialogTrigger asChild>
                  <Button size="sm">
                    <Icons.Mail className="mr-2 h-4 w-4" />
                    Invite
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Invite to team</DialogTitle>
                    <DialogDescription>
                      Send an invitation by email.
                    </DialogDescription>
                  </DialogHeader>
                  <div className="grid gap-4 py-4">
                    <div className="grid gap-2">
                      <Label>Email</Label>
                      <Input
                        type="email"
                        value={inviteEmail}
                        onChange={(e) => setInviteEmail(e.target.value)}
                        placeholder="colleague@example.com"
                        disabled={createInvitationMutation.isPending}
                      />
                    </div>
                    <div className="grid gap-2">
                      <Label>Role</Label>
                      <select
                        className="flex h-9 w-full rounded-none border border-input bg-transparent px-3 py-1 text-sm"
                        value={inviteRole}
                        onChange={(e) => setInviteRole(e.target.value)}
                        disabled={createInvitationMutation.isPending}
                      >
                        <option value="member">Member</option>
                        <option value="admin">Admin</option>
                      </select>
                    </div>
                    {createInvitationMutation.error && (
                      <p className="text-sm text-destructive">
                        {createInvitationMutation.error.message}
                      </p>
                    )}
                  </div>
                  <DialogFooter>
                    <Button
                      variant="outline"
                      onClick={() => setIsInviteOpen(false)}
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={handleCreateInvitation}
                      disabled={
                        createInvitationMutation.isPending ||
                        !inviteEmail.trim()
                      }
                    >
                      Send invite
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              {invitationsLoading ? (
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              ) : invitationsList.length === 0 ? (
                <p className="text-muted-foreground">No pending invitations.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Email</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Expires</TableHead>
                      <TableHead className="w-[80px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invitationsList.map((inv) => (
                      <TableRow key={inv.id}>
                        <TableCell>{inv.email}</TableCell>
                        <TableCell>
                          <Badge variant="secondary">{inv.role}</Badge>
                        </TableCell>
                        <TableCell>{inv.status}</TableCell>
                        <TableCell className="text-muted-foreground text-sm">
                          {new Date(inv.expires_at).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          {inv.status === "pending" && (
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => {
                                if (confirm("Revoke this invitation?")) {
                                  revokeInvitationMutation.mutate(inv.id);
                                }
                              }}
                              disabled={revokeInvitationMutation.isPending}
                            >
                              <Icons.Trash className="h-4 w-4" />
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
