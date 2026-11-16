import { type FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  AlertIcon,
  ArrowBackIcon,
  BarChartIcon,
} from "../../shared/components/Icons";
import { LoadingSpinner } from "../../shared/components/LoadingSpinner";
import { useDocumentTitle } from "../../shared/hooks/useDocumentTitle";
import {
  fetchLinkStats,
  type AnalyticsRange,
  type LinkStatsResponse,
} from "../shorten";

function formatUtcTimestamp(iso: string | undefined): string {
  if (!iso) return "Never";
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) return iso;
  return parsed.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

export function StatsPage() {
  const { slug: routeSlug } = useParams<{ slug?: string }>();
  const activeSlug = routeSlug?.trim() ?? "";

  useDocumentTitle(activeSlug ? `Stats: /${activeSlug} — MicroURL` : "Link Stats — MicroURL");

  return <StatsPageContent key={activeSlug} activeSlug={activeSlug} />;
}

function StatsPageContent({ activeSlug }: { activeSlug: string }) {
  const navigate = useNavigate();

  const [slugInput, setSlugInput] = useState(activeSlug);
  const [range, setRange] = useState<AnalyticsRange>("7d");
  const [fetchResult, setFetchResult] = useState<{
    forKey: string;
    data: LinkStatsResponse | null;
    error: string | null;
  } | null>(null);

  const requestKey = activeSlug ? `${activeSlug}:${range}` : "";

  useEffect(() => {
    if (!requestKey) {
      return;
    }

    const controller = new AbortController();

    fetchLinkStats(activeSlug, range, controller.signal)
      .then((data) => {
        setFetchResult({ forKey: requestKey, data, error: null });
      })
      .catch((err) => {
        if (controller.signal.aborted) return;
        setFetchResult({
          forKey: requestKey,
          data: null,
          error:
            err instanceof Error ? err.message : "Failed to load link stats.",
        });
      });

    return () => controller.abort();
  }, [activeSlug, range, requestKey]);

  const currentResult =
    fetchResult && fetchResult.forKey === requestKey ? fetchResult : null;
  const stats = currentResult?.data ?? null;
  const loadError = currentResult?.error ?? null;
  const loading = Boolean(requestKey) && !currentResult;

  const handleLookupSubmit = (event: FormEvent) => {
    event.preventDefault();
    const cleaned = slugInput.trim().replace(/^\/+/, "");
    if (!cleaned) return;
    navigate(`/stats/${encodeURIComponent(cleaned)}`);
  };

  const maxDailyClicks =
    stats?.daily.reduce((max, item) => Math.max(max, item.clicks), 0) ?? 0;

  return (
    <main className="dot-grid flex-grow flex flex-col w-full">
      <div className="w-full max-w-[780px] mx-auto px-4 sm:px-8 py-10 flex-grow flex flex-col gap-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <Link
              to="/"
              className="inline-flex items-center gap-1.5 text-xs text-outline hover:text-on-surface mb-2 transition-colors"
            >
              <ArrowBackIcon className="w-3.5 h-3.5" />
              <span>Back to Shortener</span>
            </Link>
            <h1 className="text-2xl font-semibold text-on-surface tracking-tight flex items-center gap-2">
              <BarChartIcon className="w-5 h-5 text-outline" />
              <span>Link Stats</span>
            </h1>
          </div>

          <form onSubmit={handleLookupSubmit} className="flex items-center gap-2 w-full sm:w-auto">
            <div className="flex items-center bg-surface border border-outline-variant focus-within:border-on-surface rounded-xl px-3 py-2 flex-grow sm:w-56 text-xs shadow-2xs transition-colors">
              <span className="text-outline font-mono mr-1 select-none">/</span>
              <input
                type="text"
                value={slugInput}
                onChange={(e) => setSlugInput(e.target.value)}
                aria-label="Short link ending"
                placeholder="e.g. my-link"
                className="w-full bg-transparent border-0 p-0 text-xs font-mono text-on-surface focus:ring-0 focus:outline-none placeholder:text-outline/55"
              />
            </div>
            <button
              type="submit"
              disabled={!slugInput.trim()}
              className="px-4 py-2 bg-on-surface text-surface text-xs font-medium rounded-xl hover:opacity-90 disabled:opacity-40 transition-opacity"
            >
              View Stats
            </button>
          </form>
        </div>

        {!activeSlug && (
          <section className="bg-surface rounded-2xl border border-outline-variant p-6 shadow-sm text-sm text-outline">
            Enter the ending of any short link above (for example, the part after /) to view daily clicks and total redirects.
          </section>
        )}

        {activeSlug && loading && (
          <section className="bg-surface rounded-2xl border border-outline-variant p-8 shadow-sm flex items-center justify-center gap-2 text-xs text-outline">
            <LoadingSpinner size="sm" />
            <span>Loading stats for /{activeSlug}...</span>
          </section>
        )}

        {activeSlug && !loading && loadError && (
          <section
            role="alert"
            className="bg-surface rounded-2xl border border-error/40 p-4 shadow-sm flex items-center gap-2 text-xs text-error"
          >
            <AlertIcon className="w-4 h-4" />
            <span>{loadError}</span>
          </section>
        )}

        {activeSlug && !loading && stats && (
          <>
            <section className="bg-surface rounded-2xl border border-outline-variant p-5 sm:p-6 shadow-sm flex flex-col gap-5">
              <div className="flex flex-wrap items-start justify-between gap-3 border-b border-outline-variant pb-4">
                <div className="min-w-0">
                  <div className="flex items-center gap-2.5 flex-wrap">
                    <span className="text-base font-mono font-bold text-on-surface">
                      /{stats.slug}
                    </span>
                    <span
                      className={`px-2.5 py-0.5 rounded-full text-[11px] font-medium capitalize border ${
                        stats.status === "active"
                          ? "border-emerald-500/40 text-emerald-500 dark:text-emerald-400"
                          : "border-error/50 text-error"
                      }`}
                    >
                      {stats.status}
                    </span>
                  </div>
                  <p
                    className="text-xs font-mono text-outline truncate max-w-md mt-1"
                    title={stats.original_url}
                  >
                    {stats.original_url}
                  </p>
                </div>

                <div className="flex items-center gap-2 flex-wrap">
                  <div
                    className="inline-flex rounded-xl bg-surface-container-low border border-outline-variant p-1 gap-1 text-xs"
                    role="group"
                    aria-label="Analytics date range"
                  >
                    {(["7d", "30d"] as const).map((option) => (
                      <button
                        key={option}
                        type="button"
                        onClick={() => setRange(option)}
                        className={
                          range === option
                            ? "px-3 py-1 rounded-lg bg-on-surface text-surface font-medium shadow-2xs"
                            : "px-3 py-1 rounded-lg text-outline hover:text-on-surface"
                        }
                      >
                        {option === "7d" ? "7 days" : "30 days"}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                <div className="bg-surface-container-low border border-outline-variant rounded-xl p-3.5">
                  <div className="text-xs text-outline">Total Clicks</div>
                  <div className="text-xl font-semibold text-on-surface mt-1 tabular-nums">
                    {stats.total_clicks}
                  </div>
                </div>
                <div className="bg-surface-container-low border border-outline-variant rounded-xl p-3.5">
                  <div className="text-xs text-outline">Time Zone</div>
                  <div className="text-sm font-medium text-on-surface mt-1.5">
                    {stats.timezone}
                  </div>
                </div>
                <div className="bg-surface-container-low border border-outline-variant rounded-xl p-3.5">
                  <div className="text-xs text-outline">Created</div>
                  <div className="text-xs text-on-surface mt-1.5">
                    {formatUtcTimestamp(stats.created_at)}
                  </div>
                </div>
                <div className="bg-surface-container-low border border-outline-variant rounded-xl p-3.5">
                  <div className="text-xs text-outline">Expires</div>
                  <div className="text-xs text-on-surface mt-1.5">
                    {formatUtcTimestamp(stats.expires_at)}
                  </div>
                </div>
              </div>

              <div className="bg-surface-container-low border border-outline-variant rounded-xl p-4">
                <div className="flex items-center justify-between text-xs text-outline mb-3">
                  <span className="font-medium text-on-surface-variant">
                    Daily Clicks ({range === "7d" ? "Last 7 days" : "Last 30 days"} · UTC)
                  </span>
                  <span>
                    {stats.total_clicks === 0
                      ? "No clicks yet"
                      : `Peak: ${maxDailyClicks}/day`}
                  </span>
                </div>

                <div
                  className="h-32 flex items-end gap-1 sm:gap-1.5 pt-4"
                  aria-label="Daily clicks trend chart"
                >
                  {stats.daily.map((day) => {
                    const ratio =
                      maxDailyClicks > 0 ? (day.clicks / maxDailyClicks) * 100 : 0;
                    const heightPct = day.clicks > 0 ? Math.max(ratio, 8) : 2;
                    return (
                      <div
                        key={day.date}
                        title={`${day.date} (UTC): ${day.clicks} clicks`}
                        className="flex-1 h-full flex flex-col justify-end items-center group"
                      >
                        <div
                          style={{ height: `${heightPct}%` }}
                          className={`w-full rounded-t-sm transition-all ${
                            day.clicks > 0
                              ? "bg-on-surface group-hover:opacity-80"
                              : "bg-outline-variant/50"
                          }`}
                        />
                      </div>
                    );
                  })}
                </div>

                {stats.daily.length > 0 && (
                  <div className="flex justify-between text-[11px] font-mono text-outline mt-2">
                    <span>{stats.daily[0].date}</span>
                    <span>{stats.daily[stats.daily.length - 1].date}</span>
                  </div>
                )}
              </div>
            </section>
          </>
        )}
      </div>
    </main>
  );
}

