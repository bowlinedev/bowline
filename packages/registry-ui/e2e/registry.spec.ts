import { expect, test } from "@playwright/test";

function renameField(value: unknown, from: string, to: string): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => renameField(item, from, to));
  }
  if (typeof value !== "object" || value === null) {
    return value;
  }
  const record = value as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const [key, child] of Object.entries(record)) {
    out[key] =
      key === "name" && child === from && record.type !== undefined
        ? to
        : renameField(child, from, to);
  }
  return out;
}

test("lists the seeded services with their latest version", async ({ page }) => {
  await page.goto("/");
  const services = page.getByTestId("services");
  await expect(services).toContainText("ledger");
  await expect(services).toContainText("billing");
  await expect(services.getByText("sha256:").first()).toBeVisible();
});

test("shows a service's versions and consumers", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("link", { name: "ledger", exact: true }).click();
  await expect(page.getByTestId("versions")).toContainText("sha256:");
  await expect(page.getByTestId("versions")).toContainText("main");
  await expect(page.getByTestId("consumers")).toContainText("ledger-web");
});

test("browses the contract of a published version", async ({ page }) => {
  await page.goto("/#/services/ledger");
  await page.getByTestId("versions").getByRole("link").first().click();
  await expect(page.getByTestId("procedures")).toContainText("invoices.get");
  await expect(page.getByText("export interface Invoice {")).toBeVisible();
});

test("draws the consumer edge in the dependency graph", async ({ page }) => {
  await page.goto("/#/graph");
  await expect(page.getByLabel("dependency graph")).toBeVisible();
  await expect(page.getByTestId("node-ledger")).toBeAttached();
  await expect(page.getByTestId("node-ledger-web")).toBeAttached();
  await expect(page.getByTestId("edge-ledger-web-ledger")).toBeAttached();
});

test("names the consumer a removed field would break", async ({ page, request }) => {
  const response = await request.get("/v1/services/ledger/latest");
  expect(response.ok()).toBe(true);
  const broken = JSON.stringify(renameField(await response.json(), "total", "amount"));

  await page.goto("/#/impact/ledger");
  await expect(page.getByLabel("Service")).toHaveValue("ledger");
  await page.getByLabel("Contract document").fill(broken);
  await page.getByRole("button", { name: "Check impact" }).click();

  const report = page.getByTestId("report");
  await expect(report).toContainText("ledger-web");
  await expect(report).toContainText("reads total");
  await expect(report).toContainText("field removed");
});
