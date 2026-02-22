import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
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
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute("/_dashboard_layout/invitations/")({
  component: InvitationsPage,
  head: () => {
    return generateMetadata({ title: "Invitations" });
  },
});

function InvitationsPage() {
  const {
    data: invitations,
    isLoading: invitationsLoading,
    error: invitationsError,
  } = usePendingInvitations();
  const acceptMutation = useAcceptInvitation();
  const declineMutation = useDeclineInvitation();

  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const list = ((invitations as Invitation[] | undefined) ?? []).filter(
    (i) => i.status === "pending",
  );

  const handleAccept = (inv: Invitation) => {
    if (!inv.accept_token) return;
    acceptMutation.mutate(inv.accept_token, {
      onSuccess: () => {
        setSuccessMessage(
          `Accepted invitation to ${inv.team_name || inv.project_name || "resource"}`,
        );
        setTimeout(() => setSuccessMessage(null), 3000);
      },
    });
  };

  const handleDecline = (inv: Invitation) => {
    if (!inv.accept_token) return;
    declineMutation.mutate(inv.accept_token, {
      onSuccess: () => {
        setSuccessMessage("Invitation declined");
        setTimeout(() => setSuccessMessage(null), 3000);
      },
    });
  };

  const loading = invitationsLoading;

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

      {successMessage ? (
        <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
          {successMessage}
        </div>
      ) : null}

      {acceptMutation.error ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {acceptMutation.error.message}
        </div>
      ) : null}

      {declineMutation.error ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {declineMutation.error.message}
        </div>
      ) : null}

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
