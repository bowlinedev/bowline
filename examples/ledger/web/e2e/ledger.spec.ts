import { expect, type Page, test } from "@playwright/test";
import { arm, collect } from "./record.js";

const uploadLimit = 8 * 1024 * 1024;

async function attachOversized(page: Page): Promise<number | string> {
  try {
    const response = await page.request.post("/api/invoices.attach", {
      multipart: {
        input: {
          name: "input",
          mimeType: "application/json",
          buffer: Buffer.from('{"invoiceId":3}'),
        },
        file: {
          name: "huge.bin",
          mimeType: "application/octet-stream",
          buffer: Buffer.alloc(uploadLimit + 1024 * 1024, 1),
        },
      },
    });
    return response.status();
  } catch {
    return "connection dropped";
  }
}

for (const transport of ["sse", "ws"]) {
  test(`lists, creates, validates, streams, attaches, and voids invoices over ${transport}`, async ({
    browser,
  }) => {
    const context = await browser.newContext();
    await arm(context);
    const page = await context.newPage();
    const observer = await context.newPage();
    const url = transport === "ws" ? "/?transport=ws" : "/";
    await page.goto(url);
    await observer.goto(url);
    await expect(page.getByTestId("health")).toHaveText(/bowline \d/);
    await expect(page.getByTestId("transport")).toContainText(`${transport} transport, live`);
    await expect(observer.getByTestId("transport")).toContainText(`${transport} transport, live`);
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
    await expect(created).toContainText("USD 30.00");
    await expect(created.getByTestId("status")).toHaveText("draft");
    const createdId = (await created.getAttribute("data-testid"))?.replace("invoice-", "");
    await expect(observer.getByTestId(`invoice-${createdId}`)).toBeVisible({ timeout: 2000 });
    await expect(observer.getByTestId("transport")).toContainText("live, 1 changes seen");

    await created.getByRole("button", { name: "void" }).click();
    await expect(created.getByTestId("status")).toHaveText("void");

    await page.getByTestId("invoice-4").getByRole("button", { name: "void" }).click();
    await expect(page.getByTestId("void-error")).toHaveText(
      "invoice 4 is locked because it is paid",
    );

    await page.getByLabel("attach to 3").setInputFiles({
      name: "receipt.bin",
      mimeType: "application/octet-stream",
      buffer: Buffer.alloc(2 * 1024 * 1024, 1),
    });
    await expect(page.getByTestId("attached")).toHaveText(
      "attached receipt.bin (2097152 bytes) to invoice 3",
    );

    expect([413, "connection dropped"]).toContain(await attachOversized(page));
    if (transport === "sse") {
      await collect(page, observer);
    }
    await context.close();
  });
}
