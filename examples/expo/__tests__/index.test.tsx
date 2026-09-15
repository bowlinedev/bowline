import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, waitFor } from "@testing-library/react-native";
import type { ReactElement } from "react";
import CreateScreen from "../app/create";
import InvoicesScreen from "../app/index";

jest.mock("expo-router", () => ({
  Link: ({ children }: { children: ReactElement }) => children,
  useRouter: () => ({ back: jest.fn() }),
}));

function respond(status: number, body: unknown): typeof fetch {
  const text = JSON.stringify(body);
  return jest.fn(async () => ({
    ok: status < 400,
    status,
    headers: new Map<string, string>(),
    text: async () => text,
  })) as unknown as typeof fetch;
}

let queryClient: QueryClient;

function renderWithQuery(element: ReactElement) {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}>{element}</QueryClientProvider>);
}

afterEach(() => {
  queryClient.clear();
});

const invoice = (id: number, status: string, total: string) => ({
  id,
  customerId: 1,
  status,
  total,
  lines: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
});

describe("invoices screen", () => {
  it("lists the invoices the API returns", async () => {
    global.fetch = respond(200, {
      items: [invoice(3, "sent", "USD 1500.00"), invoice(4, "paid", "USD 9999.00")],
    });
    const view = await renderWithQuery(<InvoicesScreen />);
    await waitFor(() => expect(view.getByTestId("invoice-3")).toBeTruthy());
    expect(view.getByTestId("invoice-4")).toBeTruthy();
    expect(view.getByTestId("status-3")).toHaveTextContent("sent");
    expect(view.getByText("USD 9999.00")).toBeTruthy();
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/invoices.list?input="),
      expect.objectContaining({ method: "GET" }),
    );
  });
});

describe("create screen", () => {
  it("renders validation issues from a 400", async () => {
    global.fetch = respond(400, {
      error: {
        code: "INVALID_ARGUMENT",
        message: "invalid input",
        issues: [
          { path: ["lines", "0", "description"], rule: "required", message: "is required" },
          { path: ["lines", "0", "quantity"], rule: "min", message: "must be at least 1" },
        ],
      },
    });
    const view = await renderWithQuery(<CreateScreen />);
    await fireEvent.changeText(view.getByTestId("quantity"), "0");
    await fireEvent.press(view.getByTestId("create-invoice"));
    await waitFor(() => expect(view.getByTestId("issues")).toBeTruthy());
    expect(view.getByText("lines.0.description: is required")).toBeTruthy();
    expect(view.getByText("lines.0.quantity: must be at least 1")).toBeTruthy();
  });
});
