import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { useAuth } from "~/lib/auth";
import { usePendingInvitations } from "~/hooks/queries";
import { useAcceptInvitation, useDeclineInvitation } from "~/hooks/mutations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";
import type { Invitation } from "~/lib/api";

export const Route = createFileRoute("/invitations/")({
  component: InvitationsPage,
});

function InvitationsPage() {
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const {
    data: invitations,
    isLoading: invitationsLoading,
    error: invitationsError,
  } = usePendingInvitations();
  const acceptMutation = useAcceptInvitation();
  const declineMutation = useDeclineInvitation();

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, authLoading, navigate]);

  const list = (invitations as Invitation[] | undefined) ?? [];
  // const pending = list.filter((i) => i.status === "pending");

  const handleAccept = (inv: Invitation) => {
    if (!inv.accept_token) return;
    acceptMutation.mutate(inv.accept_token, {
      onSuccess: () => {},
    });
  };

  const handleDecline = (inv: Invitation) => {
    if (!inv.accept_token) return;
    declineMutation.mutate(inv.accept_token, {
      onSuccess: () => {},
    });
  };

  const loading = authLoading || invitationsLoading;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (invitationsError) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>
            {(invitationsError as Error).message}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Invitations</h1>
        <p className="text-muted-foreground">
          Pending invitations to teams and projects
        </p>
      </div>

      {list.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Icons.Mail className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold">No invitations</h3>
            <p className="text-muted-foreground text-center mt-1">
              When someone invites you to a team or project, it will appear
              here.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {list.map((inv) => (
            <Card key={inv.id}>
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between">
                  <div>
                    <CardTitle className="text-base">
                      {inv.team_name
                        ? `Team: ${inv.team_name}`
                        : inv.project_name
                          ? `Project: ${inv.project_name}`
                          : "Invitation"}
                    </CardTitle>
                    <CardDescription>
                      {inv.team_id ? "Team invitation" : "Project invitation"} —{" "}
                      {inv.role}
                    </CardDescription>
                  </div>
                  <Badge
                    variant={inv.status === "pending" ? "default" : "secondary"}
                  >
                    {inv.status}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground mb-4">
                  You were invited to join{" "}
                  {inv.team_name || inv.project_name || "a resource"} as{" "}
                  {inv.role}.
                  {inv.expires_at && (
                    <>
                      {" "}
                      Expires {new Date(inv.expires_at).toLocaleDateString()}.
                    </>
                  )}
                </p>
                {inv.status === "pending" && inv.accept_token && (
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      onClick={() => handleAccept(inv)}
                      disabled={
                        acceptMutation.isPending || declineMutation.isPending
                      }
                    >
                      {acceptMutation.isPending ? (
                        <Icons.Loader className="h-4 w-4 animate-spin" />
                      ) : (
                        <>
                          <Icons.Check className="mr-1 h-4 w-4" />
                          Accept
                        </>
                      )}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleDecline(inv)}
                      disabled={
                        acceptMutation.isPending || declineMutation.isPending
                      }
                    >
                      <Icons.Close className="mr-1 h-4 w-4" />
                      Decline
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
