import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useAuth } from "~/lib/auth";
import { api } from "~/lib/api";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";

export const Route = createFileRoute("/auth/device")({
  component: DeviceAuthPage,
});

function DeviceAuthPage() {
  const navigate = useNavigate();
  const { isAuthenticated, isLoading } = useAuth();
  const [userCode, setUserCode] = useState("");
  const [status, setStatus] = useState<
    "idle" | "loading" | "success" | "error"
  >("idle");
  const [error, setError] = useState<string | null>(null);

  // Get code from URL if present
  const params = new URLSearchParams(
    typeof window !== "undefined" ? window.location.search : "",
  );
  const codeFromUrl = params.get("code");

  const handleAuthorize = async () => {
    const code = userCode || codeFromUrl;
    if (!code) {
      setError("Please enter a device code");
      return;
    }

    setStatus("loading");
    setError(null);

    try {
      await api.authorizeDevice(code.toUpperCase().trim());
      setStatus("success");
    } catch (err) {
      setStatus("error");
      setError(
        err instanceof Error ? err.message : "Failed to authorize device",
      );
    }
  };

  const handleDeny = async () => {
    const code = userCode || codeFromUrl;
    if (!code) return;

    setStatus("loading");
    try {
      await api.denyDevice(code.toUpperCase().trim());
      navigate({ to: "/" });
    } catch (err) {
      setStatus("error");
      setError(err instanceof Error ? err.message : "Failed to deny device");
    }
  };

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <CardTitle>Sign in required</CardTitle>
            <CardDescription>
              You need to sign in before authorizing a device
            </CardDescription>
          </CardHeader>
          <CardFooter>
            <Button
              className="w-full"
              onClick={() => navigate({ to: "/login" })}
            >
              Sign in
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  if (status === "success") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
              <svg
                className="h-6 w-6 text-green-600 dark:text-green-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 13l4 4L19 7"
                />
              </svg>
            </div>
            <CardTitle className="mt-4">Device Authorized</CardTitle>
            <CardDescription>
              You can now close this window and return to your terminal
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
            <svg
              className="h-6 w-6 text-primary-foreground"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>
          <CardTitle className="mt-4">Authorize Device</CardTitle>
          <CardDescription>
            Enter the code shown in your terminal to authorize the CLI
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="code">Device Code</Label>
            <Input
              id="code"
              placeholder="XXXX-XXXX"
              value={userCode || codeFromUrl || ""}
              onChange={(e) => setUserCode(e.target.value.toUpperCase())}
              className="text-center text-2xl font-mono tracking-widest"
              maxLength={9}
            />
          </div>

          {error && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}
        </CardContent>
        <CardFooter className="flex gap-3">
          <Button
            variant="outline"
            className="flex-1"
            onClick={handleDeny}
            disabled={status === "loading"}
          >
            Deny
          </Button>
          <Button
            className="flex-1"
            onClick={handleAuthorize}
            disabled={status === "loading" || (!userCode && !codeFromUrl)}
          >
            {status === "loading" ? (
              <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current" />
            ) : (
              "Authorize"
            )}
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}
