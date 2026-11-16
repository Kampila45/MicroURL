import { create } from "zustand";
import {
  createShortLink,
  ShortenerApiError,
  type CreateShortLinkResponse,
} from "../api/shortenerClient";

export type ShortenStatus = "idle" | "loading" | "success" | "error";

interface ShortenState {
  status: ShortenStatus;
  originalUrl: string | null;
  slug: string | null;
  shortUrl: string | null;
  qrCodeUrl: string | null;
  createdAt: string | null;
  expiresAt: string | null;
  error: string | null;
  errorStatusCode: number | null;
  createShortLink: (
    url: string,
    captchaToken: string,
    expiresAt?: string,
    customSlug?: string
  ) => Promise<void>;
  reset: () => void;
}

const initialState = {
  status: "idle" as ShortenStatus,
  originalUrl: null,
  slug: null,
  shortUrl: null,
  qrCodeUrl: null,
  createdAt: null,
  expiresAt: null,
  error: null,
  errorStatusCode: null,
};

export const useShortenStore = create<ShortenState>((set) => ({
  ...initialState,
  createShortLink: async (
    url: string,
    captchaToken: string,
    expiresAt?: string,
    customSlug?: string
  ) => {
    set({ status: "loading", error: null, errorStatusCode: null });

    try {
      const response: CreateShortLinkResponse = await createShortLink({
        url,
        captchaToken,
        expiresAt,
        customSlug,
      });

      set({
        status: "success",
        originalUrl: response.original_url,
        slug: response.slug,
        shortUrl: response.short_url,
        qrCodeUrl: response.qr_code_url,
        createdAt: response.created_at,
        expiresAt: response.expires_at ?? null,
        error: null,
        errorStatusCode: null,
      });
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "An unknown error occurred.";
      const statusCode =
        err instanceof ShortenerApiError ? err.statusCode : null;

      set({
        status: "error",
        error: errorMessage,
        errorStatusCode: statusCode,
        originalUrl: null,
        slug: null,
        shortUrl: null,
        qrCodeUrl: null,
        createdAt: null,
        expiresAt: null,
      });
    }
  },
  reset: () => {
    set(initialState);
  },
}));
