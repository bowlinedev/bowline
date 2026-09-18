import { expect, test } from "@playwright/test";

test("lists, validates, creates, and voids invoices against the mock with the Go server stopped", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByTestId("health")).toHaveText("bowline 1.2.0");
  await expect(page.getByTestId("invoice-3")).toBeVisible();
  await expect(page.getByTestId("invoice-4")).toBeVisible();

  await page.getByLabel("description").fill("");
  await page.getByLabel("quantity").fill("0");
  await page.getByRole("button", { name: "create invoice" }).click();
  await expect(page.getByTestId("issues")).toContainText("lines.0.description: is required");
  await expect(page.getByTestId("issues")).toContainText("lines.0.quantity: must be at least 1");

  await page.getByLabel("description").fill("Widgets");
  await page.getByLabel("quantity").fill("3");
  await page.getByRole("button", { name: "create invoice" }).click();
  const created = page.locator("[data-testid^=invoice-]").last();
  await expect(created).toContainText("USD 1500.00");
  await expect(created.getByTestId("status")).toHaveText(/draft|sent|paid|void/);
  await expect(page.locator("[data-testid^=invoice-]")).toHaveCount(6);
  await created.getByRole("button", { name: "void" }).click();
  await expect(page.locator("[data-testid^=invoice-]")).toHaveCount(6);
  await expect(page.getByTestId("void-error")).toHaveCount(0);
});
