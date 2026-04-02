import { test, expect } from "@playwright/test";

test.describe("Auth Guard", () => {
  test("redirects unauthenticated users to login from /dashboard", async ({
    page,
  }) => {
    // No tokens set — user is unauthenticated
    await page.route("**/api/v1/users/me", (route) => {
      route.fulfill({ status: 401, body: JSON.stringify({ error: "unauthorized" }) });
    });

    await page.goto("/dashboard");

    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });

  test("redirects unauthenticated users to login from /projects", async ({
    page,
  }) => {
    await page.route("**/api/v1/users/me", (route) => {
      route.fulfill({ status: 401, body: JSON.stringify({ error: "unauthorized" }) });
    });

    await page.goto("/projects");

    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });

  test("redirects unauthenticated users to login from /teams", async ({
    page,
  }) => {
    await page.route("**/api/v1/users/me", (route) => {
      route.fulfill({ status: 401, body: JSON.stringify({ error: "unauthorized" }) });
    });

    await page.goto("/teams");

    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });

  test("redirects unauthenticated users to login from /settings", async ({
    page,
  }) => {
    await page.route("**/api/v1/users/me", (route) => {
      route.fulfill({ status: 401, body: JSON.stringify({ error: "unauthorized" }) });
    });

    await page.goto("/settings");

    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });
});
