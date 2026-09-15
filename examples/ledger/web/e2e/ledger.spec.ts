import { expect, test } from "@playwright/test";

test("lists, creates, validates, and voids invoices", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("health")).toHaveText(/bowline \d/);
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
  const created = page.getByTestId("invoice-5");
  await expect(created).toBeVisible();
  await expect(created).toContainText("USD 30.00");
  await expect(created.getByTestId("status")).toHaveText("draft");

  await created.getByRole("button", { name: "void" }).click();
  await expect(created.getByTestId("status")).toHaveText("void");
  await expect(page.getByTestId("invoice-4").getByRole("button", { name: "void" })).toBeDisabled();
});
