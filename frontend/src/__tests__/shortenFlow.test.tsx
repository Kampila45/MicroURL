import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StatsPage } from "../features/analytics/StatsPage";
import { ExpiredPage } from "../features/errors/ExpiredPage";
import { NotFoundPage } from "../features/errors/NotFoundPage";
import { ServerErrorPage } from "../features/errors/ServerErrorPage";
import { HomePage } from "../features/shorten/components/HomePage";
import { useShortenStore } from "../features/shorten/store/useShortenStore";
import {
  generateQRCodeDataURL,
  generateQRCodeSVG,
} from "../shared/utils/qr";

vi.mock("../shared/utils/qr", () => ({
  generateQRCodeDataURL: vi.fn(),
  generateQRCodeSVG: vi.fn(),
  triggerFileDownload: vi.fn(),
}));

describe("MicroURL Shorten Flow", () => {
  beforeEach(() => {
    useShortenStore.getState().reset();
    vi.restoreAllMocks();
    vi.stubEnv("VITE_API_BASE_URL", "http://localhost:8080");
    vi.stubEnv("VITE_TURNSTILE_SITE_KEY", "");
    vi.mocked(generateQRCodeDataURL).mockResolvedValue(
      "data:image/png;base64,fake"
    );
    vi.mocked(generateQRCodeSVG).mockResolvedValue("<svg></svg>");
  });

  it("shows validation error for invalid URL format", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    const input = screen.getByLabelText(/enter url to shorten/i);
    await user.type(input, "not-a-valid-url");
    await user.click(screen.getByRole("button", { name: /shorten/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /invalid url format/i
    );
  });

  it("blocks unsafe schemes and non-public hosts client-side", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.spyOn(globalThis, "fetch");

    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    const input = screen.getByLabelText(/enter url to shorten/i);
    await user.type(input, "javascript:alert(1)");
    await user.click(screen.getByRole("button", { name: /shorten/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /http:\/\/ or https:\/\//i
    );

    await user.clear(input);
    await user.type(input, "http://localhost:8080/internal");
    await user.click(screen.getByRole("button", { name: /shorten/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /public destination url/i
    );

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("submits URL with optional expiration and displays result + QR tabs", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: "1-abc",
          slug: "x7k9Q2",
          original_url: "https://example.com/very/long/path",
          created_at: "2026-11-15T20:00:00Z",
          expires_at: "2026-11-16T20:00:00Z",
          short_url: "http://localhost:8080/x7k9Q2",
          qr_code_url: "http://localhost:8080/api/qr/x7k9Q2",
        }),
        { status: 201, headers: { "Content-Type": "application/json" } }
      )
    );

    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    await user.type(
      screen.getByLabelText(/enter url to shorten/i),
      "https://example.com/very/long/path"
    );
    await user.click(screen.getByRole("button", { name: "24 hours" }));
    await user.click(screen.getByRole("button", { name: /shorten/i }));

    await waitFor(() => {
      expect(screen.getByText("localhost:8080/x7k9Q2")).toBeInTheDocument();
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0];
    const sentBody = JSON.parse(String(init?.body));
    expect(sentBody.url).toBe("https://example.com/very/long/path");
    expect(sentBody.expires_at).toBeTruthy();

    await user.click(screen.getByRole("tab", { name: /qr code/i }));
    expect(
      screen.getByAltText("QR code for http://localhost:8080/x7k9Q2")
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /shorten another url/i })
    );
    expect(screen.getByLabelText(/enter url to shorten/i)).toBeInTheDocument();
  });

  it("displays rate limit (429) API error cleanly", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "rate limit exceeded" }), {
        status: 429,
      })
    );

    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    await user.type(
      screen.getByLabelText(/enter url to shorten/i),
      "https://example.com"
    );
    await user.click(screen.getByRole("button", { name: /shorten/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /too many requests/i
    );
  });

  it("renders dedicated 404, 410, and 500 error pages", () => {
    const { unmount } = render(
      <MemoryRouter initialEntries={["/unknown-route"]}>
        <Routes>
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText(/Link Not Found/)).toBeInTheDocument();
    unmount();

    const expired = render(
      <MemoryRouter initialEntries={["/410?slug=v8n2P9"]}>
        <Routes>
          <Route path="/410" element={<ExpiredPage />} />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText(/Link Expired/)).toBeInTheDocument();
    expect(screen.getByText("v8n2P9")).toBeInTheDocument();
    expired.unmount();

    render(
      <MemoryRouter initialEntries={["/500"]}>
        <Routes>
          <Route path="/500" element={<ServerErrorPage />} />
        </Routes>
      </MemoryRouter>
    );
    expect(screen.getByText(/Service Unavailable/)).toBeInTheDocument();
  });

  it("supports custom link endings and conflict errors", async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "conflict" }), { status: 409 })
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: "2-def",
            slug: "portfolio-2026",
            original_url: "https://example.com/portfolio",
            created_at: "2026-11-15T20:00:00Z",
            short_url: "http://localhost:8080/portfolio-2026",
            qr_code_url: "http://localhost:8080/api/qr/portfolio-2026",
          }),
          { status: 201, headers: { "Content-Type": "application/json" } }
        )
      );

    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    await user.type(
      screen.getByLabelText(/enter url to shorten/i),
      "https://example.com/portfolio"
    );
    await user.type(
      screen.getByLabelText(/custom link/i),
      "Portfolio-2026"
    );
    await user.click(screen.getByRole("button", { name: /shorten/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /that custom link is already taken/i
    );

    await user.click(screen.getByRole("button", { name: /shorten/i }));
    await waitFor(() => {
      expect(
        screen.getByText("localhost:8080/portfolio-2026")
      ).toBeInTheDocument();
    });

    // Normalized lowercase custom ending sent to API.
    const [, secondInit] = fetchMock.mock.calls[1];
    expect(JSON.parse(String(secondInit?.body)).custom_slug).toBe(
      "portfolio-2026"
    );
  });

  it("loads link stats dashboard", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          slug: "portfolio-2026",
          original_url: "https://example.com/portfolio",
          status: "active",
          created_at: "2026-11-15T20:00:00Z",
          total_clicks: 12,
          timezone: "UTC",
          range_days: 7,
          daily: [
            { date: "2026-11-14", clicks: 5 },
            { date: "2026-11-15", clicks: 7 },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      )
    );

    render(
      <MemoryRouter initialEntries={["/stats/portfolio-2026"]}>
        <Routes>
          <Route path="/stats/:slug" element={<StatsPage />} />
        </Routes>
      </MemoryRouter>
    );

    expect(await screen.findByText("/portfolio-2026")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("UTC")).toBeInTheDocument();
  });
});

