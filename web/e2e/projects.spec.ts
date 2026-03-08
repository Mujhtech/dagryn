import {
  test,
  expect,
  setupAuthenticatedSession,
  setupDashboardMocks,
  mockProjects,
} from "./fixtures";

test.describe("Projects Page", () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthenticatedSession(page);
    await setupDashboardMocks(page);
  });

  test("renders projects list with heading", async ({ page }) => {
    await page.goto("/projects");

    await expect(
      page.getByRole("heading", { name: "Projects" }),
    ).toBeVisible();
    await expect(
      page.getByText("Manage your workflow projects"),
    ).toBeVisible();
  });

  test("displays project cards", async ({ page }) => {
    await page.goto("/projects");

    await expect(page.getByText("My Project")).toBeVisible();
    await expect(page.getByText("my-project")).toBeVisible();
    await expect(page.getByText("Public App")).toBeVisible();
    await expect(page.getByText("public-app")).toBeVisible();
  });

  test("shows visibility badges", async ({ page }) => {
    await page.goto("/projects");

    await expect(page.getByText("private")).toBeVisible();
    await expect(page.getByText("public")).toBeVisible();
  });

  test("shows member count on project cards", async ({ page }) => {
    await page.goto("/projects");

    await expect(page.getByText("2 members")).toBeVisible();
    await expect(page.getByText("5 members")).toBeVisible();
  });

  test("shows empty state when no projects", async ({ page }) => {
    await page.route("**/api/v1/projects**", (route) => {
      if (route.request().method() === "GET") {
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            data: [],
            meta: { total: 0, page: 1, per_page: 20 },
          }),
        });
      } else {
        route.continue();
      }
    });

    await page.goto("/projects");

    await expect(page.getByText("No projects yet")).toBeVisible();
    await expect(
      page.getByText("Create your first project to get started"),
    ).toBeVisible();
  });

  test("has create project button", async ({ page }) => {
    await page.goto("/projects");

    await expect(
      page.getByRole("button", { name: /new project/i }),
    ).toBeVisible();
  });

  test("has import from GitHub button", async ({ page }) => {
    await page.goto("/projects");

    await expect(
      page.getByRole("link", { name: /import from github/i }),
    ).toBeVisible();
  });

  test("opens create project dialog", async ({ page }) => {
    await page.goto("/projects");

    await page.getByRole("button", { name: /new project/i }).click();

    await expect(
      page.getByRole("heading", { name: "Create Project" }),
    ).toBeVisible();
    await expect(page.getByLabel("Name")).toBeVisible();
    await expect(page.getByLabel("Slug")).toBeVisible();
  });

  test("create project dialog auto-generates slug from name", async ({
    page,
  }) => {
    await page.goto("/projects");

    await page.getByRole("button", { name: /new project/i }).click();
    await page.getByLabel("Name").fill("My New Project");

    const slugInput = page.getByLabel("Slug");
    await expect(slugInput).toHaveValue("my-new-project");
  });

  test("create project dialog validates required fields", async ({ page }) => {
    await page.goto("/projects");

    await page.getByRole("button", { name: /new project/i }).click();

    // Submit empty form
    await page.getByRole("button", { name: "Create Project" }).click();

    await expect(page.getByText("Project name is required")).toBeVisible();
  });

  test("create project dialog submits successfully", async ({ page }) => {
    const newProject = {
      id: "proj-new",
      name: "Brand New",
      slug: "brand-new",
      description: "Fresh project",
      visibility: "private",
      member_count: 1,
      created_at: "2025-06-15T00:00:00Z",
      updated_at: "2025-06-15T00:00:00Z",
    };

    await page.route("**/api/v1/projects", (route) => {
      if (route.request().method() === "POST") {
        route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify(newProject),
        });
      } else {
        route.continue();
      }
    });

    // Mock the project detail page endpoint
    await page.route(`**/api/v1/projects/${newProject.id}**`, (route) => {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: newProject }),
      });
    });

    await page.goto("/projects");

    await page.getByRole("button", { name: /new project/i }).click();
    await page.getByLabel("Name").fill("Brand New");
    await page.getByLabel(/description/i).fill("Fresh project");
    await page.getByRole("button", { name: "Create Project" }).click();

    // Should navigate to the new project
    await expect(page).toHaveURL(/\/projects\/proj-new/, { timeout: 10000 });
  });

  test("shows error message on project creation failure", async ({ page }) => {
    await page.route("**/api/v1/projects", (route) => {
      if (route.request().method() === "POST") {
        route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({
            error: "project with this slug already exists",
          }),
        });
      } else {
        route.continue();
      }
    });

    await page.goto("/projects");

    await page.getByRole("button", { name: /new project/i }).click();
    await page.getByLabel("Name").fill("Existing Project");
    await page.getByRole("button", { name: "Create Project" }).click();

    // Error should be displayed in the dialog
    await expect(
      page.locator(".bg-destructive\\/10"),
    ).toBeVisible({ timeout: 5000 });
  });
});
