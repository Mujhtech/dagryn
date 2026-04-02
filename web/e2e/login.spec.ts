import { test, expect } from "@playwright/test";

test.describe("Login Page", () => {
  test("renders login page with branding", async ({ page }) => {
    await page.goto("/login");

    await expect(page.getByRole("heading", { name: "Dagryn" })).toBeVisible();
    await expect(
      page.getByText("Local-first workflow runtime"),
    ).toBeVisible();
  });

  test("shows OAuth provider buttons when providers load", async ({
    page,
  }) => {
    await page.route("**/api/v1/auth/providers", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: [
            {
              id: "github",
              name: "GitHub",
              display_name: "GitHub",
              auth_url: "https://github.com/login/oauth/authorize?client_id=test",
              enabled: true,
            },
            {
              id: "google",
              name: "Google",
              display_name: "Google",
              auth_url: "https://accounts.google.com/o/oauth2/auth?client_id=test",
              enabled: true,
            },
          ],
        }),
      });
    });

    await page.goto("/login");

    await expect(
      page.getByRole("button", { name: /continue with github/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /continue with google/i }),
    ).toBeVisible();
  });

  test("shows CLI auth alternative", async ({ page }) => {
    await page.route("**/api/v1/auth/providers", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: [] }),
      });
    });

    await page.goto("/login");

    await expect(page.getByText("dagryn auth login")).toBeVisible();
  });

  test("shows terms of service and privacy policy links", async ({ page }) => {
    await page.route("**/api/v1/auth/providers", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: [] }),
      });
    });

    await page.goto("/login");

    await expect(
      page.getByRole("link", { name: /terms of service/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: /privacy policy/i }),
    ).toBeVisible();
  });

  test("redirects to dashboard if already authenticated", async ({ page }) => {
    // Set up auth before navigating
    await page.addInitScript(() => {
      localStorage.setItem("access_token", "mock-access-token");
      localStorage.setItem("refresh_token", "mock-refresh-token");
    });

    await page.route("**/api/v1/users/me", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            id: "user-001",
            email: "test@example.com",
            name: "Test User",
            created_at: "2025-01-01T00:00:00Z",
          },
        }),
      });
    });

    await page.route("**/api/v1/license/status", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ mode: "cloud", valid: true }),
      });
    });

    await page.route("**/api/v1/dashboard/overview", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ projects: [], recent_runs: [] }),
      });
    });

    await page.goto("/login");

    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10000 });
  });
});
