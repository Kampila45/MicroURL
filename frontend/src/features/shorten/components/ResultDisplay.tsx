import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  BarChartIcon,
  CheckIcon,
  CopyIcon,
  DownloadIcon,
  ExternalLinkIcon,
  ImageIcon,
  LinkIcon,
  QrCodeIcon,
  ResetIcon,
  ShareIcon,
  SplitColumnsIcon,
} from "../../../shared/components/Icons";
import { LoadingSpinner } from "../../../shared/components/LoadingSpinner";
import {
  generateQRCodeDataURL,
  generateQRCodeSVG,
  triggerFileDownload,
} from "../../../shared/utils/qr";
import { useShortenStore } from "../store/useShortenStore";

export type ResultViewMode = "link" | "qr" | "both";

function formatDisplayTimestamp(isoString: string | null): string {
  if (!isoString) {
    return "Just now";
  }
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) {
    return isoString;
  }
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function stripProtocol(url: string): { protocol: string; rest: string } {
  const match = /^(https?:\/\/)(.+)$/i.exec(url);
  if (!match) {
    return { protocol: "", rest: url };
  }
  return { protocol: match[1], rest: match[2] };
}

export function ResultDisplay() {
  const {
    status,
    shortUrl,
    originalUrl,
    slug,
    createdAt,
    expiresAt,
    reset,
  } = useShortenStore();

  const [viewMode, setViewMode] = useState<ResultViewMode>("both");
  const [copied, setCopied] = useState(false);
  const [qrState, setQrState] = useState<{
    forUrl: string;
    pngUrl: string | null;
    svg: string | null;
    failed: boolean;
  } | null>(null);

  useEffect(() => {
    if (status !== "success" || !shortUrl) {
      return;
    }

    let cancelled = false;

    Promise.all([
      generateQRCodeDataURL(shortUrl),
      generateQRCodeSVG(shortUrl),
    ])
      .then(([pngDataUrl, svgString]) => {
        if (!cancelled) {
          setQrState({
            forUrl: shortUrl,
            pngUrl: pngDataUrl,
            svg: svgString,
            failed: false,
          });
        }
      })
      .catch((err) => {
        console.error("Failed to generate QR code:", err);
        if (!cancelled) {
          setQrState({
            forUrl: shortUrl,
            pngUrl: null,
            svg: null,
            failed: true,
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [status, shortUrl]);

  const currentQr = qrState && qrState.forUrl === shortUrl ? qrState : null;
  const qrPngUrl = currentQr?.pngUrl ?? null;
  const qrSvg = currentQr?.svg ?? null;
  const isGeneratingQr = status === "success" && Boolean(shortUrl) && !currentQr;

  useEffect(() => {
    if (status !== "success") return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        reset();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [status, reset]);

  if (status !== "success" || !shortUrl) {
    return null;
  }

  const { protocol, rest } = stripProtocol(shortUrl);
  const fileBase = `microurl-${slug || "qr"}`;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(shortUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy link:", err);
    }
  };

  const handleShare = async () => {
    if (navigator.share) {
      try {
        await navigator.share({
          title: "MicroURL Short Link",
          url: shortUrl,
        });
        return;
      } catch {
        // User cancelled or Web Share unavailable; fall back to copy.
      }
    }
    await handleCopy();
  };

  const handleDownloadPng = () => {
    if (!qrPngUrl) return;
    triggerFileDownload(qrPngUrl, `${fileBase}.png`);
  };

  const handleDownloadSvg = () => {
    if (!qrSvg) return;
    const blob = new Blob([qrSvg], { type: "image/svg+xml;charset=utf-8" });
    const objectUrl = URL.createObjectURL(blob);
    triggerFileDownload(objectUrl, `${fileBase}.svg`);
    URL.revokeObjectURL(objectUrl);
  };

  const showLinkColumn = viewMode === "link" || viewMode === "both";
  const showQrColumn = viewMode === "qr" || viewMode === "both";

  return (
    <section
      aria-label="Shortened link result"
      className="w-full bg-surface rounded-2xl border border-outline-variant p-5 sm:p-6 shadow-sm flex flex-col gap-5 relative"
    >
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-outline-variant pb-3.5">
        <div
          className="inline-flex rounded-xl bg-surface-container-low border border-outline-variant p-1 gap-1"
          role="tablist"
          aria-label="Result view mode"
        >
          <button
            type="button"
            role="tab"
            aria-selected={viewMode === "link"}
            onClick={() => setViewMode("link")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
              viewMode === "link"
                ? "text-surface bg-on-surface shadow-2xs"
                : "text-outline hover:text-on-surface"
            }`}
          >
            <LinkIcon className="w-3.5 h-3.5" />
            <span>Short Link</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={viewMode === "qr"}
            onClick={() => setViewMode("qr")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
              viewMode === "qr"
                ? "text-surface bg-on-surface shadow-2xs"
                : "text-outline hover:text-on-surface"
            }`}
          >
            <QrCodeIcon className="w-3.5 h-3.5" />
            <span>QR Code</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={viewMode === "both"}
            onClick={() => setViewMode("both")}
            className={`hidden md:flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
              viewMode === "both"
                ? "text-surface bg-on-surface shadow-2xs"
                : "text-outline hover:text-on-surface"
            }`}
          >
            <SplitColumnsIcon className="w-3.5 h-3.5" />
            <span>Side-by-Side</span>
          </button>
        </div>

        <button
          type="button"
          onClick={reset}
          className="flex items-center gap-1.5 text-xs text-outline hover:text-on-surface transition-colors py-1.5 px-3 rounded-lg border border-transparent hover:border-outline-variant hover:bg-surface-container-low"
        >
          <ResetIcon className="w-3.5 h-3.5" />
          <span>Shorten Another URL</span>
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-12 gap-5 items-stretch">
        {showLinkColumn && (
          <div
            className={`${
              viewMode === "both" ? "md:col-span-7" : "md:col-span-12"
            } flex flex-col justify-between space-y-4`}
          >
            <div className="space-y-2.5">
              <div className="flex items-center justify-between text-xs">
                <span className="text-on-surface-variant font-medium">
                  Short Link
                </span>
                <span className="text-emerald-500 dark:text-emerald-400 flex items-center gap-1.5 font-medium">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                  Ready
                </span>
              </div>

              <div className="bg-surface-container-low rounded-xl border border-outline-variant p-3.5 flex items-center justify-between gap-2">
                <div className="flex items-center min-w-0 font-mono text-base sm:text-lg">
                  {protocol && (
                    <span className="text-outline select-none shrink-0">
                      {protocol}
                    </span>
                  )}
                  <span className="text-on-surface font-semibold tracking-tight truncate">
                    {rest}
                  </span>
                </div>
                <a
                  href={shortUrl}
                  target="_blank"
                  rel="noreferrer"
                  title="Open in new tab"
                  aria-label="Open shortened URL in new tab"
                  className="p-2 text-outline hover:text-on-surface hover:bg-surface-container rounded-lg transition-colors shrink-0"
                >
                  <ExternalLinkIcon className="w-4 h-4" />
                </a>
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={handleCopy}
                  className={`flex-grow py-2.5 px-4 rounded-xl flex items-center justify-center gap-2 transition-all active:scale-[0.99] text-xs font-medium ${
                    copied
                      ? "bg-emerald-500 text-white"
                      : "bg-on-surface text-surface hover:opacity-90"
                  }`}
                >
                  {copied ? (
                    <CheckIcon className="w-4 h-4" />
                  ) : (
                    <CopyIcon className="w-4 h-4" />
                  )}
                  <span>{copied ? "Copied to Clipboard" : "Copy Link"}</span>
                </button>
                <button
                  type="button"
                  onClick={handleShare}
                  title="Share link"
                  aria-label="Share shortened link"
                  className="bg-surface-container-low border border-outline-variant text-on-surface hover:bg-surface-container py-2.5 px-3.5 rounded-xl flex items-center justify-center transition-colors"
                >
                  <ShareIcon className="w-4 h-4" />
                </button>
              </div>
            </div>

            <div className="bg-surface-container-low rounded-xl p-3.5 border border-outline-variant flex flex-col gap-2 text-xs">
              <div className="flex items-center justify-between gap-3">
                <span className="text-outline">Destination</span>
                <span
                  className="text-on-surface-variant font-mono text-[11px] truncate max-w-[260px]"
                  title={originalUrl ?? undefined}
                >
                  {originalUrl}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-outline">Expires</span>
                <span className="text-on-surface-variant text-xs">
                  {expiresAt
                    ? formatDisplayTimestamp(expiresAt)
                    : "Never"}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-outline">Created</span>
                <span className="text-on-surface-variant text-xs">
                  {formatDisplayTimestamp(createdAt)}
                </span>
              </div>
              {slug && (
                <div className="flex items-center justify-between gap-3 pt-2 border-t border-outline-variant/60">
                  <span className="text-outline">Analytics</span>
                  <Link
                    to={`/stats/${encodeURIComponent(slug)}`}
                    className="inline-flex items-center gap-1.5 text-xs font-medium text-on-surface hover:underline"
                  >
                    <BarChartIcon className="w-3.5 h-3.5" />
                    <span>View Link Stats →</span>
                  </Link>
                </div>
              )}
            </div>
          </div>
        )}

        {showQrColumn && (
          <div
            className={`${
              viewMode === "both"
                ? "md:col-span-5 border-t md:border-t-0 md:border-l border-outline-variant pt-4 md:pt-0 md:pl-5"
                : "md:col-span-12 py-2"
            } flex flex-col items-center justify-center`}
          >
            <div className="flex flex-col items-center space-y-3 w-full max-w-[220px]">
              <div className="flex items-center justify-between w-full text-xs text-outline">
                <span className="font-medium text-on-surface-variant">QR Code</span>
                <span>Scan to open</span>
              </div>

              <div className="w-full aspect-square bg-[#09090B] p-3.5 rounded-xl border border-outline-variant flex items-center justify-center">
                {isGeneratingQr ? (
                  <LoadingSpinner size="md" className="text-white" />
                ) : qrPngUrl ? (
                  <img
                    src={qrPngUrl}
                    alt={`QR code for ${shortUrl}`}
                    className="w-full h-full object-contain"
                  />
                ) : (
                  <span className="text-xs text-outline">
                    QR unavailable
                  </span>
                )}
              </div>

              <div className="flex w-full gap-2 pt-0.5">
                <button
                  type="button"
                  disabled={!qrSvg}
                  onClick={handleDownloadSvg}
                  className="flex-1 bg-surface-container-low hover:bg-surface-container border border-outline-variant text-on-surface py-2 rounded-xl text-xs font-medium transition-colors flex items-center justify-center gap-1.5 disabled:opacity-45"
                >
                  <DownloadIcon className="w-3.5 h-3.5" />
                  <span>SVG</span>
                </button>
                <button
                  type="button"
                  disabled={!qrPngUrl}
                  onClick={handleDownloadPng}
                  className="flex-1 bg-surface-container-low hover:bg-surface-container border border-outline-variant text-on-surface py-2 rounded-xl text-xs font-medium transition-colors flex items-center justify-center gap-1.5 disabled:opacity-45"
                >
                  <ImageIcon className="w-3.5 h-3.5" />
                  <span>PNG</span>
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
