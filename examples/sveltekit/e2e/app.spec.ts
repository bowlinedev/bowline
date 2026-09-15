import { expect, test } from "@playwright/test";

test("renders the list on the server, creates in the browser, and shows issues", async ({
  page,
}) => {
  const html = await (await page.request.get("/")).text();
  expect(html).toContain('data-testid="invoice-3"');
  expect(html).toContain('data-testid="invoice-4"');

  await page.goto("/");
  await expect(page.getByTestId("health")).toHaveText(/bowline \d/);
  await expect(page.getByTestId("invoice-3")).toBeVisible();

  await page.getByLabel("description").fill("");
  await page.getByLabel("quantity").fill("0");
  await page.getByRole("button", { name: "create invoice" }).click();
  await expect(page.getByTestId("issues")).toContainText("lines.0.description: is required");
  await expect(page.getByTestId("issues")).toContainText("lines.0.quantity: must be at least 1");

  await page.getByLabel("description").fill("Widgets");
  await page.getByLabel("quantity").fill("3");
  await page.getByRole("button", { name: "create invoice" }).click();
  const created = page.locator("[data-testid^=invoice-]").last();
  await expect(created).toContainText("USD 30.00");
  await expect(created.getByTestId("status")).toHaveText("draft");
  await expect(page.getByTestId("issues")).toHaveCount(0);
});
