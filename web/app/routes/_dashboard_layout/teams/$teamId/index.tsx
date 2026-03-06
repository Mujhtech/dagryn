import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import {
  useTeam,
  useTeamMembers,
  useTeamInvitations,
  useTeamAuditLogs,
  useAuditWebhooks,
} from "~/hooks/queries";
import {
  useUpdateTeam,
  useDeleteTeam,
  useCreateTeamInvitation,
  useRevokeTeamInvitation,
  useRemoveTeamMember,
  useCreateAuditWebhook,
  useUpdateAuditWebhook,
  useDeleteAuditWebhook,
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
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
import { Switch } from "~/components/ui/switch";
import type { Team, TeamMember, Invitation, AuditLogEntry, AuditWebhook } from "~/lib/api";
import { api } from "~/lib/api";
import { Icons } from "~/components/icons";
import { AuditLogDetailSheet } from "~/components/audit-log-detail";
import { generateMetadata } from "~/lib/metadata";
import type { AuditLogParams } from "~/hooks/queries/use-team-audit-logs";

export const Route = createFileRoute("/_dashboard_layout/teams/$teamId/")({
  component: TeamDetailPage,
  head: () => {
    return generateMetadata({ title: "Team" });
  },
});

function TeamDetailPage() {
  const { teamId } = Route.useParams();
  const navigate = useNavigate();

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

  const loading = teamLoading;
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
          <Button variant="outline" asChild>
            <Link
              to="/teams/$teamId/analytics"
              params={{ teamId }}
            >
              <Icons.TrendUp className="mr-2 h-4 w-4" />
              Analytics
            </Link>
          </Button>
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
          <TabsTrigger value="audit-log">
            <Icons.ClipboardList className="mr-2 h-4 w-4" />
            Audit Log
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
        <TabsContent value="audit-log" className="mt-4">
          <AuditLogTab teamId={teamId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

const CATEGORY_OPTIONS = [
  { value: "", label: "All categories" },
  { value: "auth", label: "Auth" },
  { value: "team", label: "Team" },
  { value: "project", label: "Project" },
  { value: "member", label: "Member" },
  { value: "billing", label: "Billing" },
  { value: "audit", label: "Audit" },
];

function categoryColor(category: string) {
  switch (category) {
    case "auth":
      return "default";
    case "team":
      return "secondary";
    case "project":
      return "outline";
    case "member":
      return "secondary";
    case "billing":
      return "destructive";
    case "audit":
      return "outline";
    default:
      return "secondary";
  }
}

function formatRelativeTime(dateStr: string) {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return "just now";
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

function AuditLogTab({ teamId }: { teamId: string }) {
  const [category, setCategory] = useState("");
  const [actorEmail, setActorEmail] = useState("");
  const [since, setSince] = useState("");
  const [until, setUntil] = useState("");
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [allEntries, setAllEntries] = useState<AuditLogEntry[]>([]);
  const [isExporting, setIsExporting] = useState(false);
  const [selectedEntry, setSelectedEntry] = useState<AuditLogEntry | null>(null);
  const [showAddWebhook, setShowAddWebhook] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookDescription, setWebhookDescription] = useState("");
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [testingWebhookId, setTestingWebhookId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ webhookId: string; success: boolean; statusCode?: number; error?: string; durationMs: number } | null>(null);

  const params: AuditLogParams = {
    ...(category ? { category } : {}),
    ...(actorEmail ? { actor_email: actorEmail } : {}),
    ...(since ? { since: new Date(since).toISOString() } : {}),
    ...(until ? { until: new Date(until).toISOString() } : {}),
    ...(cursor ? { cursor } : {}),
    limit: 50,
  };

  const { data, isLoading, error } = useTeamAuditLogs(teamId, params);
  const { data: webhooks = [], isLoading: webhooksLoading } =
    useAuditWebhooks(teamId);

  const createWebhookMutation = useCreateAuditWebhook(teamId);
  const updateWebhookMutation = useUpdateAuditWebhook(teamId);
  const deleteWebhookMutation = useDeleteAuditWebhook(teamId);

  const handleCreateWebhook = async () => {
    if (!webhookUrl) return;
    try {
      const result = await createWebhookMutation.mutateAsync({
        url: webhookUrl,
        description: webhookDescription || undefined,
      });
      setCreatedSecret(result.secret);
      setWebhookUrl("");
      setWebhookDescription("");
    } catch {
      // mutation error is available via createWebhookMutation.error
    }
  };

  const handleToggleWebhook = (webhook: AuditWebhook) => {
    updateWebhookMutation.mutate({
      webhookId: webhook.id,
      data: { is_active: !webhook.is_active },
    });
  };

  const handleDeleteWebhook = (webhookId: string) => {
    deleteWebhookMutation.mutate(webhookId);
  };

  const handleTestWebhook = async (webhookId: string) => {
    setTestingWebhookId(webhookId);
    setTestResult(null);
    try {
      const { data } = await api.testAuditWebhook(teamId, webhookId);
      setTestResult({
        webhookId,
        success: data.success,
        statusCode: data.status_code,
        error: data.error,
        durationMs: data.duration_ms,
      });
    } catch {
      setTestResult({ webhookId, success: false, error: "Request failed", durationMs: 0 });
    } finally {
      setTestingWebhookId(null);
    }
  };

  // Accumulate entries for "load more" pagination
  useEffect(() => {
    if (data) {
      if (cursor) {
        setAllEntries((prev) => {
          const existingIds = new Set(prev.map((e) => e.id));
          const newEntries = data.data.filter((e) => !existingIds.has(e.id));
          return [...prev, ...newEntries];
        });
      } else {
        setAllEntries(data.data);
      }
    }
  }, [data, cursor]);

  // Reset entries when filters change
  const handleFilterChange = useCallback(() => {
    setCursor(undefined);
    setAllEntries([]);
  }, []);

  const handleExport = async (format: "csv" | "json") => {
    setIsExporting(true);
    try {
      const result = await api.exportTeamAuditLogs(teamId, format, {
        ...(category ? { category } : {}),
        ...(actorEmail ? { actor_email: actorEmail } : {}),
        ...(since ? { since: new Date(since).toISOString() } : {}),
        ...(until ? { until: new Date(until).toISOString() } : {}),
      });
      const url = URL.createObjectURL(result.blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = result.filename || `audit-logs.${format}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch {
      // Silently fail — user sees no file downloaded
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <>
    <Card className="py-3">
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>Audit Log</CardTitle>
          <CardDescription>Activity history for this team</CardDescription>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleExport("csv")}
            disabled={isExporting}
          >
            <Icons.Download className="mr-2 h-4 w-4" />
            CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleExport("json")}
            disabled={isExporting}
          >
            <Icons.Download className="mr-2 h-4 w-4" />
            JSON
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {/* Filter bar */}
        <div className="flex flex-wrap gap-3 mb-4">
          <Select
            value={category}
            onValueChange={(value) => {
              setCategory(value === "all" ? "" : value);
              handleFilterChange();
            }}
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="All categories" />
            </SelectTrigger>
            <SelectContent>
              {CATEGORY_OPTIONS.map((opt) => (
                <SelectItem key={opt.value || "all"} value={opt.value || "all"}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="relative">
            <Icons.Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Filter by actor email"
              className="pl-8 h-9 w-[220px]"
              value={actorEmail}
              onChange={(e) => {
                setActorEmail(e.target.value);
                handleFilterChange();
              }}
            />
          </div>
          <Input
            type="date"
            className="h-9 w-[160px]"
            value={since}
            onChange={(e) => {
              setSince(e.target.value);
              handleFilterChange();
            }}
            placeholder="Since"
          />
          <Input
            type="date"
            className="h-9 w-[160px]"
            value={until}
            onChange={(e) => {
              setUntil(e.target.value);
              handleFilterChange();
            }}
            placeholder="Until"
          />
        </div>

        {/* Table */}
        {isLoading && !allEntries.length ? (
          <div className="flex items-center justify-center h-32">
            <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <p className="text-sm text-destructive">{(error as Error).message}</p>
        ) : allEntries.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            No audit log entries found.
          </p>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Actor</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Resource</TableHead>
                  <TableHead>IP Address</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {allEntries.map((entry) => (
                  <TableRow
                    key={entry.id}
                    className="cursor-pointer hover:bg-muted/50"
                    onClick={() => setSelectedEntry(entry)}
                  >
                    <TableCell
                      className="text-muted-foreground text-sm whitespace-nowrap"
                      title={new Date(entry.created_at).toLocaleString()}
                    >
                      {formatRelativeTime(entry.created_at)}
                    </TableCell>
                    <TableCell>
                      <div className="text-sm">
                        {entry.actor_type === "system"
                          ? "System"
                          : entry.actor_email || "Unknown"}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          categoryColor(entry.category) as
                            | "default"
                            | "secondary"
                            | "outline"
                            | "destructive"
                        }
                      >
                        {entry.action}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm">
                      {entry.resource_type}
                      {entry.resource_id
                        ? ` (${entry.resource_id.slice(0, 8)}...)`
                        : ""}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {entry.ip_address || "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {/* Load more */}
            {data?.has_next && (
              <div className="flex justify-center mt-4">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCursor(data.next_cursor)}
                  disabled={isLoading}
                >
                  {isLoading ? (
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                  ) : null}
                  Load more
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>

    {/* Webhooks Section */}
    <Card className="py-3 mt-6">
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>Webhooks</CardTitle>
          <CardDescription>
            Forward audit log events to external SIEM systems
          </CardDescription>
        </div>
        <Dialog open={showAddWebhook} onOpenChange={(open) => {
          setShowAddWebhook(open);
          if (!open) {
            setCreatedSecret(null);
            setWebhookUrl("");
            setWebhookDescription("");
          }
        }}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Icons.Plus className="mr-2 h-4 w-4" />
              Add Webhook
            </Button>
          </DialogTrigger>
          <DialogContent>
            {createdSecret ? (
              <>
                <DialogHeader>
                  <DialogTitle>Webhook Created</DialogTitle>
                  <DialogDescription>
                    Copy the signing secret below. It will not be shown again.
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-2">
                  <Label>Signing Secret</Label>
                  <div className="flex gap-2">
                    <Input
                      readOnly
                      value={createdSecret}
                      className="font-mono text-sm"
                    />
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => navigator.clipboard.writeText(createdSecret)}
                    >
                      Copy
                    </Button>
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={() => setShowAddWebhook(false)}>Done</Button>
                </DialogFooter>
              </>
            ) : (
              <>
                <DialogHeader>
                  <DialogTitle>Add Webhook</DialogTitle>
                  <DialogDescription>
                    Configure a webhook endpoint to receive audit log events via
                    HTTPS POST with HMAC-SHA256 signing.
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="webhook-url">URL</Label>
                    <Input
                      id="webhook-url"
                      placeholder="https://example.com/webhook"
                      value={webhookUrl}
                      onChange={(e) => setWebhookUrl(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="webhook-desc">Description (optional)</Label>
                    <Input
                      id="webhook-desc"
                      placeholder="e.g. Splunk SIEM"
                      value={webhookDescription}
                      onChange={(e) => setWebhookDescription(e.target.value)}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button
                    variant="outline"
                    onClick={() => setShowAddWebhook(false)}
                  >
                    Cancel
                  </Button>
                  <Button
                    onClick={handleCreateWebhook}
                    disabled={!webhookUrl || createWebhookMutation.isPending}
                  >
                    {createWebhookMutation.isPending ? (
                      <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    ) : null}
                    Create
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent>
        {webhooksLoading ? (
          <div className="flex items-center justify-center h-20">
            <Icons.Loader className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : webhooks.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            No webhooks configured.
          </p>
        ) : (
          <div className="space-y-3">
            {webhooks.map((webhook) => (
              <div
                key={webhook.id}
                className="flex items-center justify-between rounded-md border p-3"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium truncate">
                      {webhook.url}
                    </span>
                    <Badge variant={webhook.is_active ? "default" : "secondary"}>
                      {webhook.is_active ? "Active" : "Inactive"}
                    </Badge>
                  </div>
                  {webhook.description && (
                    <p className="text-xs text-muted-foreground mt-1">
                      {webhook.description}
                    </p>
                  )}
                </div>
                <div className="flex items-center gap-3 ml-4">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleTestWebhook(webhook.id)}
                    disabled={testingWebhookId === webhook.id}
                  >
                    {testingWebhookId === webhook.id ? (
                      <Icons.Loader className="mr-1 h-3 w-3 animate-spin" />
                    ) : null}
                    Test
                  </Button>
                  <Switch
                    size="sm"
                    checked={webhook.is_active}
                    onCheckedChange={() => handleToggleWebhook(webhook)}
                    disabled={updateWebhookMutation.isPending}
                  />
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleDeleteWebhook(webhook.id)}
                    disabled={deleteWebhookMutation.isPending}
                  >
                    <Icons.Trash className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
                {testResult && testResult.webhookId === webhook.id && (
                  <div className={`text-xs mt-2 p-2 rounded ${testResult.success ? "bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300" : "bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300"}`}>
                    {testResult.success
                      ? `Delivered successfully (HTTP ${testResult.statusCode}, ${testResult.durationMs}ms)`
                      : `Failed: ${testResult.error || `HTTP ${testResult.statusCode}`} (${testResult.durationMs}ms)`}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>

    <AuditLogDetailSheet
      entry={selectedEntry}
      open={!!selectedEntry}
      onOpenChange={(open) => { if (!open) setSelectedEntry(null); }}
    />
    </>
  );
}
