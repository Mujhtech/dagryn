import { useState, useEffect } from "react";
import { createFileRoute } from "@tanstack/react-router";

import { useAuth } from "~/lib/auth";
import { useUpdateUser } from "~/hooks/mutations";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Avatar, AvatarFallback, AvatarImage } from "~/components/ui/avatar";
import { Separator } from "~/components/ui/separator";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/settings")({
  component: SettingsPage,
});

function SettingsPage() {
  const { user, refreshUser } = useAuth();
  const [name, setName] = useState("");
  const [saveSuccess, setSaveSuccess] = useState(false);

  // Use TanStack Query mutation for updating user
  const updateUserMutation = useUpdateUser();

  useEffect(() => {
    if (user) {
      setName(user.name || "");
    }
  }, [user]);

  const handleSave = async () => {
    if (!name.trim()) {
      return;
    }

    updateUserMutation.mutate(
      { name: name.trim() },
      {
        onSuccess: async () => {
          await refreshUser();
          setSaveSuccess(true);
          setTimeout(() => setSaveSuccess(false), 3000);
        },
      },
    );
  };

  if (!user) {
    return null;
  }

  return (
    <div className="container max-w-2xl py-8 px-6">
      <div className="mb-8">
        <h1 className="text-3xl font-bold">Settings</h1>
        <p className="text-muted-foreground">
          Manage your account settings and preferences.
        </p>
      </div>

      <div className="space-y-6">
        {/* Profile Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Icons.User className="h-5 w-5" />
              Profile
            </CardTitle>
            <CardDescription>Update your personal information.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Avatar Display */}
            <div className="flex items-center gap-4">
              <Avatar className="h-20 w-20">
                <AvatarImage src={user.avatar_url} alt={user.name || "User"} />
                <AvatarFallback className="text-2xl">
                  {user.name?.charAt(0)?.toUpperCase() || "U"}
                </AvatarFallback>
              </Avatar>
              <div>
                <p className="text-sm text-muted-foreground">
                  Profile picture is synced from your OAuth provider.
                </p>
              </div>
            </div>

            <Separator />

            {/* Name Field */}
            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Your name"
              />
            </div>

            {/* Email Field (Read-only) */}
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <div className="flex items-center gap-2">
                <Icons.Mail className="h-4 w-4 text-muted-foreground" />
                <Input
                  id="email"
                  value={user.email}
                  disabled
                  className="bg-muted"
                />
              </div>
              <p className="text-xs text-muted-foreground">
                Email is managed by your OAuth provider and cannot be changed.
              </p>
            </div>

            {updateUserMutation.error && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {updateUserMutation.error.message}
              </div>
            )}

            {saveSuccess && (
              <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
                Profile updated successfully!
              </div>
            )}
          </CardContent>
          <CardFooter>
            <Button
              onClick={handleSave}
              disabled={updateUserMutation.isPending}
            >
              {updateUserMutation.isPending ? (
                <>
                  <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Icons.FloppyDisk className="mr-2 h-4 w-4" />
                  Save Changes
                </>
              )}
            </Button>
          </CardFooter>
        </Card>

        {/* Account Info Card */}
        <Card>
          <CardHeader>
            <CardTitle>Account Information</CardTitle>
            <CardDescription>Details about your account.</CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="space-y-4 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">User ID</dt>
                <dd className="font-mono">{user.id}</dd>
              </div>
              <Separator />
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Member Since</dt>
                <dd>
                  {new Date(user.created_at).toLocaleDateString("en-US", {
                    year: "numeric",
                    month: "long",
                    day: "numeric",
                  })}
                </dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        {/* Danger Zone */}
        <Card className="border-destructive/50">
          <CardHeader>
            <CardTitle className="text-destructive">Danger Zone</CardTitle>
            <CardDescription>
              Irreversible and destructive actions.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between">
              <div>
                <p className="font-medium">Delete Account</p>
                <p className="text-sm text-muted-foreground">
                  Permanently delete your account and all associated data.
                </p>
              </div>
              <Button variant="destructive" disabled>
                Delete Account
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
