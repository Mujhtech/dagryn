import { test, expect } from "@playwright/test";

test.describe("Landing Page", () => {
  test("renders hero section with title and CTA buttons", async ({ page }) => {
    await page.goto("/");

    await expect(
      page.getByRole("heading", {
        name: /build and ship software/i,
      }),
    ).toBeVisible();

    await expect(page.getByRole("link", { name: /start building/i })).toBeVisible();
    await expect(
      page.getByRole("link", { name: /import from github/i }),
    ).toBeVisible();
  });

  test("displays feature highlights", async ({ page }) => {
    await page.goto("/");

    await expect(page.getByText("Deterministic by default")).toBeVisible();
    await expect(page.getByText("Local-first speed")).toBeVisible();
    await expect(page.getByText("Simple workflow model")).toBeVisible();
  });

  test("Start Building link navigates to login", async ({ page }) => {
    await page.goto("/");

    await page.getByRole("link", { name: /start building/i }).click();
    await expect(page).toHaveURL(/\/login/);
  });

  test("shows Dagryn branding", async ({ page }) => {
    await page.goto("/");

    await expect(
      page.getByText("Local-first workflow runtime"),
    ).toBeVisible();
  });
});
