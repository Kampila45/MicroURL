export type ApiHealthStatus = "checking" | "healthy" | "degraded" | "unhealthy";
export type LinkStatus = "active" | "expired";
export type AnalyticsRange = "7d" | "30d";

export interface CreateShortLinkRequest {
  url: string;
  captchaToken: string;
  expiresAt?: string;
  customSlug?: string;
}

export interface CreateShortLinkResponse {
  id: string;
  slug: string;
  original_url: string;
  created_at: string;
  expires_at?: string;
  short_url: string;
  qr_code_url: string;
}

export interface DailyClickCount {
  date: string;
  clicks: number;
}

export interface LinkStatsResponse {
  slug: string;
  original_url: string;
  status: LinkStatus;
  created_at: string;
  expires_at?: string;
  total_clicks: number;
  timezone: string;
  range_days: number;
  daily: DailyClickCount[];
}

export interface HealthResponse {
  status: "healthy" | "degraded" | "unhealthy";
  message: string;
  database: {
    status: string;
    message: string;
  };
  cache: {
    status: string;
    message: string;
  };
  timestamp: string;
}

export class ShortenerApiError extends Error {
  readonly statusCode: number;

  constructor(message: string, statusCode: number) {
    super(message);
    this.name = "ShortenerApiError";
    this.statusCode = statusCode;
  }
}

export function getApiBaseUrl(): string {
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL;
  if (!apiBaseUrl) {
    throw new ShortenerApiError(
      "VITE_API_BASE_URL environment variable is not configured.",
      0
    );
  }
  return apiBaseUrl.replace(/\/+$/, "");
}

const ERROR_MESSAGE_BY_STATUS: Record<number, string> = {
  400: "Please check your URL, custom link ending, or expiration date.",
  403: "Verification failed. Please complete the check again.",
  404: "Short link not found.",
  409: "That custom link is already taken. Please try another.",
  413: "That URL is too long to submit.",
  429: "Too many requests. Please wait a moment before trying again.",
  500: "Server error. Please try again shortly.",
};

export async function createShortLink(
  request: CreateShortLinkRequest
): Promise<CreateShortLinkResponse> {
  const apiBaseUrl = getApiBaseUrl();

  const body: {
    url: string;
    captcha_token: string;
    expires_at?: string;
    custom_slug?: string;
  } = {
    url: request.url,
    captcha_token: request.captchaToken,
  };

  if (request.expiresAt) {
    body.expires_at = request.expiresAt;
  }
  if (request.customSlug) {
    body.custom_slug = request.customSlug;
  }

  const response = await fetch(`${apiBaseUrl}/api/links`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    const message =
      response.status === 409 && !request.customSlug
        ? "Could not generate a unique short link. Please try again."
        : (ERROR_MESSAGE_BY_STATUS[response.status] ??
          `Failed to create short link (HTTP ${response.status}).`);
    throw new ShortenerApiError(message, response.status);
  }

  return (await response.json()) as CreateShortLinkResponse;
}

export async function fetchLinkStats(
  slug: string,
  range: AnalyticsRange = "7d",
  signal?: AbortSignal
): Promise<LinkStatsResponse> {
  const apiBaseUrl = getApiBaseUrl();
  const response = await fetch(
    `${apiBaseUrl}/api/links/${encodeURIComponent(slug)}/stats?range=${range}`,
    {
      method: "GET",
      signal,
    }
  );

  if (!response.ok) {
    const message =
      ERROR_MESSAGE_BY_STATUS[response.status] ??
      `Failed to load stats (HTTP ${response.status}).`;
    throw new ShortenerApiError(message, response.status);
  }

  return (await response.json()) as LinkStatsResponse;
}

export async function fetchApiHealth(
  signal?: AbortSignal
): Promise<HealthResponse | null> {
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL;
  if (!apiBaseUrl) {
    return null;
  }

  const normalized = apiBaseUrl.replace(/\/+$/, "");
  const response = await fetch(`${normalized}/health`, {
    method: "GET",
    signal,
  });

  if (!response.ok && response.status !== 503) {
    return null;
  }

  return (await response.json()) as HealthResponse;
}

