import { type ChangeEvent, type FormEvent, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { AlertIcon, CheckCheckIcon } from "../../../shared/components/Icons";
import { LoadingSpinner } from "../../../shared/components/LoadingSpinner";
import { useTheme } from "../../../shared/hooks/useTheme";
import { useShortenStore } from "../store/useShortenStore";
import { CaptchaField, type CaptchaFieldRef } from "./CaptchaField";

export type ExpirationPreset = "never" | "24h" | "7d" | "custom";

const EXPIRATION_OPTIONS: ReadonlyArray<{
  value: ExpirationPreset;
  label: string;
}> = [
  { value: "never", label: "Never" },
  { value: "24h", label: "24 hours" },
  { value: "7d", label: "7 days" },
  { value: "custom", label: "Custom" },
];

function resolveExpirationIso(
  preset: ExpirationPreset,
  customDateTime: string
): { iso?: string; error?: string } {
  if (preset === "never") {
    return {};
  }
  const now = Date.now();
  if (preset === "24h") {
    return { iso: new Date(now + 24 * 60 * 60 * 1000).toISOString() };
  }
  if (preset === "7d") {
    return { iso: new Date(now + 7 * 24 * 60 * 60 * 1000).toISOString() };
  }
  if (!customDateTime) {
    return { error: "Please select a custom expiration date and time." };
  }
  const parsed = new Date(customDateTime);
  if (Number.isNaN(parsed.getTime()) || parsed.getTime() <= now) {
    return { error: "Custom expiration must be a valid future date and time." };
  }
  return { iso: parsed.toISOString() };
}

const MAX_URL_LENGTH = 2048;
const MIN_ALIAS_LENGTH = 3;
const MAX_ALIAS_LENGTH = 64;

const RESERVED_ALIASES = new Set([
  "api",
  "health",
  "docs",
  "openapi.json",
  "robots.txt",
  "favicon.ico",
  "favicon.svg",
  "favicon-32x32.png",
  "assets",
  "stats",
  "analytics",
  "manage",
  "404",
  "410",
  "500",
]);

function validateCustomAlias(raw: string): {
  normalized?: string;
  error?: string;
} {
  const normalized = raw.trim().toLowerCase();
  if (!normalized) {
    return {};
  }
  if (normalized.length < MIN_ALIAS_LENGTH || normalized.length > MAX_ALIAS_LENGTH) {
    return {
      error: `Custom ending must be between ${MIN_ALIAS_LENGTH} and ${MAX_ALIAS_LENGTH} characters.`,
    };
  }
  if (RESERVED_ALIASES.has(normalized)) {
    return { error: "That custom ending is reserved and cannot be used." };
  }
  if (!/^[a-z0-9][a-z0-9_-]*[a-z0-9]$/.test(normalized)) {
    return {
      error:
        "Custom ending may only use letters, numbers, hyphens, and underscores, and must start and end with a letter or number.",
    };
  }
  return { normalized };
}

function isPrivateOrLocalHostname(hostname: string): boolean {
  const bare = hostname.replace(/^\[|\]$/g, "").toLowerCase();
  if (!bare) {
    return true;
  }

  // IPv6 loopback, unspecified, link-local, or unique-local addresses.
  if (bare.includes(":")) {
    return (
      bare === "::" ||
      bare === "::1" ||
      bare.startsWith("fe80:") ||
      bare.startsWith("fc") ||
      bare.startsWith("fd") ||
      bare.startsWith("::ffff:127.") ||
      bare.startsWith("::ffff:10.") ||
      bare.startsWith("::ffff:192.168.") ||
      bare.startsWith("::ffff:169.254.")
    );
  }

  const ipv4Match = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(bare);
  if (ipv4Match) {
    const [, a, b] = ipv4Match.map(Number);
    if (
      a === 0 ||
      a === 10 ||
      a === 127 ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && b === 168)
    ) {
      return true;
    }
    return false;
  }

  if (
    bare === "localhost" ||
    bare.endsWith(".localhost") ||
    bare.endsWith(".local") ||
    bare.endsWith(".internal") ||
    !bare.includes(".")
  ) {
    return true;
  }

  return false;
}

function validateHttpUrl(input: string): string | null {
  const trimmed = input.trim();
  if (!trimmed) {
    return "URL is required.";
  }
  if (trimmed.length > MAX_URL_LENGTH) {
    return `URL must be ${MAX_URL_LENGTH} characters or fewer.`;
  }
  try {
    const parsed = new URL(trimmed);
    if (
      (parsed.protocol !== "http:" && parsed.protocol !== "https:") ||
      !parsed.hostname
    ) {
      return "Please enter a valid http:// or https:// URL.";
    }
    if (parsed.username || parsed.password) {
      return "URLs with embedded credentials are not allowed.";
    }
    if (isPrivateOrLocalHostname(parsed.hostname)) {
      return "Please enter a public destination URL.";
    }
    return null;
  } catch {
    return "Invalid URL format.";
  }
}

export function ShortenForm() {
  const [url, setUrl] = useState("");
  const [customAlias, setCustomAlias] = useState("");
  const [expirationPreset, setExpirationPreset] =
    useState<ExpirationPreset>("never");
  const [customExpiration, setCustomExpiration] = useState("");
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);
  const captchaRef = useRef<CaptchaFieldRef>(null);

  const { theme } = useTheme();
  const {
    createShortLink,
    status,
    originalUrl: submittedUrl,
    error,
    errorStatusCode,
  } = useShortenStore();

  const captchaRequired = Boolean(import.meta.env.VITE_TURNSTILE_SITE_KEY);
  const isSubmitting = status === "loading";
  const isSuccess = status === "success";

  // Global Cmd/Ctrl+V shortcut focuses the URL input when invoked outside inputs.
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "v") {
        const activeTag = document.activeElement?.tagName;
        if (activeTag !== "INPUT" && activeTag !== "TEXTAREA") {
          inputRef.current?.focus();
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const handleUrlChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextUrl = event.target.value;
    setUrl(nextUrl);
    if (validationError) {
      setValidationError(validateHttpUrl(nextUrl));
    }
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();

    const urlIssue = validateHttpUrl(url);
    if (urlIssue) {
      setValidationError(urlIssue);
      return;
    }

    const { iso: expiresAtIso, error: expirationError } = resolveExpirationIso(
      expirationPreset,
      customExpiration
    );
    if (expirationError) {
      setValidationError(expirationError);
      return;
    }

    const { normalized: normalizedAlias, error: aliasError } =
      validateCustomAlias(customAlias);
    if (aliasError) {
      setValidationError(aliasError);
      return;
    }

    if (captchaRequired && !captchaToken) {
      setValidationError("Please complete the verification check.");
      return;
    }

    setValidationError(null);
    await createShortLink(
      url.trim(),
      captchaToken ?? "",
      expiresAtIso,
      normalizedAlias
    );
    setCaptchaToken(null);
    captchaRef.current?.reset();
  };

  if (isSuccess && submittedUrl) {
    return (
      <section className="w-full bg-surface rounded-2xl border border-outline-variant p-4 shadow-xs transition-all">
        <div className="flex flex-col gap-2.5">
          <div className="flex items-center justify-between text-xs">
            <span className="text-outline font-medium flex items-center gap-2">
              <span className="w-1.5 h-1.5 rounded-full bg-on-surface" />
              Destination URL
            </span>
            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full bg-surface-container-low border border-outline-variant text-on-surface-variant text-[11px] font-medium">
              <span>{captchaRequired ? "Verified" : "Validated"}</span>
            </span>
          </div>

          <div className="flex flex-col sm:flex-row gap-2.5">
            <div className="relative flex-grow flex items-center bg-surface-container-low rounded-xl border border-outline-variant px-3.5 py-2.5">
              <input
                type="text"
                readOnly
                value={submittedUrl}
                aria-label="Submitted original URL"
                className="w-full bg-transparent border-0 p-0 text-xs sm:text-sm font-mono text-on-surface focus:ring-0 focus:outline-none truncate"
              />
            </div>
            <div className="bg-on-surface text-surface px-4 py-2.5 rounded-xl font-medium text-xs flex items-center justify-center gap-1.5 whitespace-nowrap">
              <CheckCheckIcon className="w-4 h-4" />
              <span>Shortened</span>
            </div>
          </div>
        </div>
      </section>
    );
  }

  return (
    <div className="w-full">
      <form
        className="bg-surface rounded-2xl border border-outline-variant p-5 sm:p-6 shadow-sm space-y-5"
        onSubmit={handleSubmit}
        noValidate
      >
        <div className="space-y-1.5">
          <label
            htmlFor="url-input"
            className="block text-xs font-medium text-on-surface-variant"
          >
            Destination URL
          </label>
          <input
            ref={inputRef}
            id="url-input"
            type="url"
            value={url}
            onChange={handleUrlChange}
            disabled={isSubmitting}
            autoComplete="off"
            aria-label="Enter URL to shorten"
            placeholder="https://example.com/your-long-url"
            className="w-full bg-surface-container-low rounded-xl border border-outline-variant focus:border-on-surface text-on-surface font-mono text-xs sm:text-sm py-3 px-3.5 focus:ring-0 focus:outline-none placeholder:text-outline/55 disabled:opacity-60 transition-colors"
          />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label
              htmlFor="alias-input"
              className="block text-xs font-medium text-on-surface-variant"
            >
              Custom link{" "}
              <span className="text-outline font-normal">(optional)</span>
            </label>
            <div className="flex items-center rounded-xl bg-surface-container-low border border-outline-variant focus-within:border-on-surface transition-colors px-3 py-2.5 min-w-0">
              <span className="text-outline font-mono text-xs sm:text-sm mr-1 select-none">
                /
              </span>
              <input
                id="alias-input"
                type="text"
                value={customAlias}
                onChange={(event) => {
                  setCustomAlias(event.target.value);
                  if (validationError) {
                    setValidationError(null);
                  }
                }}
                disabled={isSubmitting}
                maxLength={MAX_ALIAS_LENGTH}
                autoComplete="off"
                spellCheck={false}
                placeholder="launch-2026"
                className="w-full bg-transparent border-0 p-0 text-xs sm:text-sm font-mono text-on-surface focus:ring-0 focus:outline-none placeholder:text-outline/55 disabled:opacity-60"
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <span className="block text-xs font-medium text-on-surface-variant">
              Expires
            </span>
            <div
              className="grid grid-cols-4 gap-1 rounded-xl bg-surface-container-low border border-outline-variant p-1 text-xs"
              role="group"
              aria-label="Link expiration"
            >
              {EXPIRATION_OPTIONS.map((option) => {
                const active = expirationPreset === option.value;
                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => {
                      setExpirationPreset(option.value);
                      setValidationError(null);
                    }}
                    className={
                      active
                        ? "py-1.5 px-1.5 rounded-lg bg-on-surface text-surface font-medium shadow-2xs transition-colors whitespace-nowrap"
                        : "py-1.5 px-1.5 rounded-lg text-outline hover:text-on-surface transition-colors whitespace-nowrap"
                    }
                  >
                    {option.label}
                  </button>
                );
              })}
            </div>
          </div>
        </div>

        {expirationPreset === "custom" && (
          <div className="space-y-1.5">
            <label
              htmlFor="custom-expires-at"
              className="block text-xs font-medium text-on-surface-variant"
            >
              Expiration date &amp; time
            </label>
            <input
              id="custom-expires-at"
              type="datetime-local"
              value={customExpiration}
              onChange={(event) => {
                setCustomExpiration(event.target.value);
                setValidationError(null);
              }}
              className="w-full rounded-xl border border-outline-variant focus:border-on-surface bg-surface-container-low px-3.5 py-2.5 text-on-surface text-xs font-mono focus:ring-0 focus:outline-none transition-colors"
            />
          </div>
        )}

        <CaptchaField
          ref={captchaRef}
          theme={theme}
          onToken={(token) => {
            setCaptchaToken(token);
            setValidationError(null);
          }}
          onError={() => {
            setCaptchaToken(null);
            setValidationError("Verification check failed. Please try again.");
          }}
          onExpire={() => {
            setCaptchaToken(null);
          }}
        />

        {(validationError || (status === "error" && error)) && (
          <div
            role="alert"
            className="flex items-center justify-between gap-3 rounded-xl bg-error-container/25 border border-error/40 px-3.5 py-2.5 text-xs text-error"
          >
            <div className="flex items-center gap-2">
              <AlertIcon className="w-4 h-4 shrink-0" />
              <span>{validationError ?? error}</span>
            </div>
            {errorStatusCode === 500 && (
              <Link
                to="/500"
                className="shrink-0 font-medium underline hover:opacity-80"
              >
                Details →
              </Link>
            )}
          </div>
        )}

        <button
          type="submit"
          disabled={isSubmitting || url.trim() === ""}
          className="w-full h-11 px-5 bg-on-surface text-surface text-sm font-medium rounded-xl hover:opacity-90 transition-all duration-150 flex items-center justify-center gap-2 active:scale-[0.99] disabled:opacity-40 disabled:cursor-not-allowed shadow-xs"
        >
          {isSubmitting ? (
            <>
              <LoadingSpinner size="sm" />
              <span>Shortening...</span>
            </>
          ) : (
            <span>Shorten link</span>
          )}
        </button>
      </form>
    </div>
  );
}
