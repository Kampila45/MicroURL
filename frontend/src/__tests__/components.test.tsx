import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StatsPage } from "../features/analytics/StatsPage";
import { HomePage } from "../features/shorten/components/HomePage";
import { ResultDisplay } from "../features/shorten/components/ResultDisplay";
import { useShortenStore } from "../features/shorten/store/useShortenStore";
import { TopNavBar } from "../shared/components/TopNavBar";
import {
  generateQRCodeDataURL,
  generateQRCodeSVG,
  triggerFileDownload,
} from "../shared/utils/qr";

vi.mock("../shared/utils/qr", () => ({
  generateQRCodeDataURL: vi.fn(),
  generateQRCodeSVG: vi.fn(),
  triggerFileDownload: vi.fn(),
}));

describe("MicroURL Components & Edge Flows", () => {
  beforeEach(() => {
    useShortenStore.getState().reset();
    localStorage.clear();
    document.documentElement.classList.remove("dark");
    document.documentElement.removeAttribute("data-theme");
    vi.restoreAllMocks();
    vi.stubEnv("VITE_API_BASE_URL", "http://localhost:8080");
    vi.stubEnv("VITE_TURNSTILE_SITE_KEY", "");
    vi.mocked(generateQRCodeDataURL).mockResolvedValue(
      "data:image/png;base64,fake"
    );
    vi.mocked(generateQRCodeSVG).mockResolvedValue("<svg></svg>");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("copies short link, downloads SVG/PNG QR codes, and resets on Escape", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    const createObjectURL = vi.fn(() => "blob:mock-qr");
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true,
      writable: true,
      value: createObjectURL,
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true,
      writable: true,
      value: revokeObjectURL,
    });

    useShortenStore.setState({
      status: "success",
      originalUrl: "https://example.com/article",
      slug: "launch-26",
      shortUrl: "http://localhost:8080/launch-26",
      qrCodeUrl: "http://localhost:8080/api/qr/launch-26",
      createdAt: "2026-11-16T10:00:00Z",
      expiresAt: null,
    });

    render(
      <MemoryRouter>
        <ResultDisplay />
      </MemoryRouter>
    );

    await user.click(screen.getByRole("button", { name: /copy link/i }));
    expect(writeText).toHaveBeenCalledWith("http://localhost:8080/launch-26");
    expect(
      await screen.findByRole("button", { name: /copied to clipboard/i })
    ).toBeInTheDocument();

    const pngButton = await screen.findByRole("button", { name: /png/i });
    const svgButton = screen.getByRole("button", { name: /svg/i });
    await waitFor(() => {
      expect(pngButton).toBeEnabled();
      expect(svgButton).toBeEnabled();
    });

    await user.click(pngButton);
    expect(triggerFileDownload).toHaveBeenCalledWith(
      "data:image/png;base64,fake",
      "microurl-launch-26.png"
    );

    await user.click(svgButton);
    expect(triggerFileDownload).toHaveBeenCalledWith(
      "blob:mock-qr",
      "microurl-launch-26.svg"
    );

    await user.keyboard("{Escape}");
    expect(useShortenStore.getState().status).toBe("idle");
  });

  it("toggles light/dark theme in TopNavBar and resets store on logo click", async () => {
    const user = userEvent.setup();
    const onResetHome = vi.fn();

    render(
      <MemoryRouter>
        <TopNavBar onResetHome={onResetHome} />
      </MemoryRouter>
    );

    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    await user.click(
      screen.getByRole("button", { name: /switch to light mode/i })
    );
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(localStorage.getItem("theme")).toBe("light");

    await user.click(screen.getByRole("link", { name: /microurl home/i }));
    expect(onResetHome).toHaveBeenCalledTimes(1);
  });

  it("switches stats range between 7d and 30d and shows missing-slug error", async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            slug: "docs",
            original_url: "https://example.com/docs",
            status: "active",
            created_at: "2026-11-15T20:00:00Z",
            total_clicks: 5,
            timezone: "UTC",
            range_days: 7,
            daily: [{ date: "2026-11-16", clicks: 5 }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        )
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            slug: "docs",
            original_url: "https://example.com/docs",
            status: "active",
            created_at: "2026-11-15T20:00:00Z",
            total_clicks: 18,
            timezone: "UTC",
            range_days: 30,
            daily: [{ date: "2026-11-16", clicks: 18 }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        )
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "not found" }), { status: 404 })
      );

    render(
      <MemoryRouter initialEntries={["/stats/docs"]}>
        <Routes>
          <Route path="/stats/:slug" element={<StatsPage />} />
        </Routes>
      </MemoryRouter>
    );

    expect(await screen.findByText("/docs")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "30 days" }));
    await waitFor(() => {
      expect(screen.getByText("18")).toBeInTheDocument();
    });
    expect(String(fetchMock.mock.calls[1][0])).toContain("range=30d");

    const lookupInput = screen.getByLabelText(/short link ending/i);
    await user.clear(lookupInput);
    await user.type(lookupInput, "missing-link");
    await user.click(screen.getByRole("button", { name: /view stats/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent(/not found/i);
  });

  it("blocks submit when Turnstile site key is set without token or Custom expiry is empty", async () => {
    const user = userEvent.setup();
    vi.stubEnv("VITE_TURNSTILE_SITE_KEY", "1x00000000000000000000AA");

    render(
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    );

    await user.type(
      screen.getByLabelText(/enter url to shorten/i),
      "https://example.com"
    );
    await user.click(screen.getByRole("button", { name: "Custom" }));
    await user.click(screen.getByRole("button", { name: /shorten/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /select a custom expiration date/i
    );

    await user.click(screen.getByRole("button", { name: "Never" }));
    await user.click(screen.getByRole("button", { name: /shorten/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /verification check/i
    );
  });
});
