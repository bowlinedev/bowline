import { expect, test } from "@playwright/test";

test("renders the seeded invoices on the server before hydration", async ({ page }) => {
  const response = await page.request.get("/");
  const html = await response.text();
  expect(html).toContain('data-testid="invoice-3"');
  expect(html).toContain('data-testid="invoice-4"');
  expect(html).toContain("invoices rendered on the server");
});

test("creates through the proxied client and revalidates the loader", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("health")).toHaveText(/bowline \d/);
  const before = await page.locator("[data-testid^=invoice-]").count();

  await page.getByLabel("description").fill("");
  await page.getByLabel("quantity").fill("0");
  await page.getByRole("button", { name: "create invoice" }).click();
  await expect(page.getByTestId("issues")).toContainText("lines.0.description: is required");
  await expect(page.getByTestId("issues")).toContainText("lines.0.quantity: must be at least 1");

  await page.getByLabel("description").fill("Widgets");
  await page.getByLabel("quantity").fill("3");
  await page.getByRole("button", { name: "create invoice" }).click();
  await expect(page.getByTestId("created")).toContainText("created invoice");
  await expect(page.locator("[data-testid^=invoice-]")).toHaveCount(before + 1);
  const created = page.locator("[data-testid^=invoice-]").last();
  await expect(created).toContainText("USD 30.00");
  await expect(created.getByTestId("status")).toHaveText("draft");
});
