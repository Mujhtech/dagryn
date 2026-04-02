import {
  test,
  expect,
  setupAuthenticatedSession,
  setupDashboardMocks,
} from "./fixtures";

test.describe("Sidebar Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthenticatedSession(page);
    await setupDashboardMocks(page);

    await page.route("**/api/v1/dashboard/overview", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ projects: [], recent_runs: [] }),
      });
    });

    await page.route("**/api/v1/invitations/pending**", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: [], meta: { total: 0 } }),
      });
    });

    await page.route("**/api/v1/users/me/settings", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: { theme: "system" } }),
      });
    });
  });

  test("sidebar shows main navigation links", async ({ page }) => {
    await page.goto("/dashboard");

    // Check sidebar navigation items exist
    const sidebar = page.locator("[data-sidebar]").first();
    await expect(sidebar.getByText("Dashboard")).toBeVisible();
    await expect(sidebar.getByText("Projects")).toBeVisible();
  });

  test("navigates from dashboard to projects", async ({ page }) => {
    await page.goto("/dashboard");

    const sidebar = page.locator("[data-sidebar]").first();
    await sidebar.getByRole("link", { name: "Projects" }).click();

    await expect(page).toHaveURL(/\/projects/);
  });

  test("navigates from projects back to dashboard", async ({ page }) => {
    await page.goto("/projects");

    const sidebar = page.locator("[data-sidebar]").first();
    await sidebar.getByRole("link", { name: "Dashboard" }).click();

    await expect(page).toHaveURL(/\/dashboard/);
  });

  test("shows breadcrumb on dashboard", async ({ page }) => {
    await page.goto("/dashboard");

    await expect(page.getByText("Dashboard").first()).toBeVisible();
  });

  test("shows breadcrumb on projects page", async ({ page }) => {
    await page.goto("/projects");

    // The breadcrumb should show "Projects"
    const header = page.locator("header").first();
    await expect(header.getByText("Projects")).toBeVisible();
  });
});
