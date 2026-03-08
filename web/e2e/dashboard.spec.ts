import {
  test,
  expect,
  setupAuthenticatedSession,
  setupDashboardMocks,
} from "./fixtures";

test.describe("Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthenticatedSession(page);
    await setupDashboardMocks(page);

    await page.route("**/api/v1/dashboard/overview", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          projects: [
            {
              id: "proj-001",
              name: "My Project",
              slug: "my-project",
              description: "A test project",
              visibility: "private",
              member_count: 2,
              created_at: "2025-01-01T00:00:00Z",
              updated_at: "2025-06-01T00:00:00Z",
            },
          ],
          recent_runs: [
            {
              id: "run-001",
              project_id: "proj-001",
              project_name: "My Project",
              status: "success",
              started_at: "2025-06-01T10:00:00Z",
              finished_at: "2025-06-01T10:05:00Z",
              duration_ms: 300000,
              task_count: 5,
            },
          ],
        }),
      });
    });
  });

  test("renders dashboard with heading", async ({ page }) => {
    await page.goto("/dashboard");

    await expect(
      page.getByRole("heading", { name: "Dashboard" }),
    ).toBeVisible();
    await expect(
      page.getByText("Quick overview of all your projects"),
    ).toBeVisible();
  });

  test("shows project cards when projects exist", async ({ page }) => {
    await page.goto("/dashboard");

    await expect(page.getByText("My Project")).toBeVisible();
  });

  test("shows setup guide when no projects exist", async ({ page }) => {
    await page.route("**/api/v1/dashboard/overview", (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ projects: [], recent_runs: [] }),
      });
    });

    await page.goto("/dashboard");

    await expect(page.getByText("1. Install Dagryn CLI")).toBeVisible();
    await expect(page.getByText("2. Initialize Dagryn")).toBeVisible();
    await expect(page.getByText("3. Run your workflow")).toBeVisible();
  });

  test("has navigation links to projects and plugins", async ({ page }) => {
    await page.goto("/dashboard");

    await expect(
      page.getByRole("link", { name: /manage projects/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: /browse plugins/i }),
    ).toBeVisible();
  });

  test("navigates to projects page", async ({ page }) => {
    await page.goto("/dashboard");

    await page.getByRole("link", { name: /manage projects/i }).click();
    await expect(page).toHaveURL(/\/projects/);
  });
});
