import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { api } from "~/lib/api";

export const Route = createFileRoute("/sso-callback")({
  component: SSOCallbackPage,
});

function SSOCallbackPage() {
  const navigate = useNavigate();

  useEffect(() => {
    // Read tokens from URL hash fragment
    const hash = window.location.hash.substring(1);
    const params = new URLSearchParams(hash);

    const accessToken = params.get("access_token");
    const refreshToken = params.get("refresh_token");

    if (accessToken && refreshToken) {
      // Store tokens — setToken updates both localStorage and the in-memory api client
      api.setToken(accessToken);
      localStorage.setItem("refresh_token", refreshToken);

      // Clear the hash
      window.location.hash = "";

      // Redirect to dashboard
      navigate({ to: "/" });
    } else {
      // No tokens — redirect to login
      navigate({ to: "/login" });
    }
  }, [navigate]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <h2 className="text-lg font-semibold">Completing SSO sign-in...</h2>
        <p className="text-muted-foreground">
          You will be redirected shortly.
        </p>
      </div>
    </div>
  );
}
