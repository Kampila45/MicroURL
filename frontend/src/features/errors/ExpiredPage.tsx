import { Link, useSearchParams } from "react-router-dom";
import { AddLinkIcon, InfoIcon } from "../../shared/components/Icons";
import { ErrorPageShell } from "./ErrorPageShell";

export function ExpiredPage() {
  const [searchParams] = useSearchParams();
  const slug = searchParams.get("slug");
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL;

  return (
    <ErrorPageShell
      title="410 Link Expired — MicroURL"
      badgeText="410 · Link Expired"
      badgeVariant="warning"
      heading="This short link has expired."
      description={
        slug ? (
          <>
            Short link <code className="text-on-surface">{slug}</code> reached
            its expiration time and is no longer active.
          </>
        ) : (
          "This short link reached its expiration time and is no longer active."
        )
      }
      actions={
        <>
          <Link
            to="/"
            className="inline-flex items-center justify-center gap-2 h-10 px-4 rounded-xl bg-on-surface text-surface text-xs font-medium hover:opacity-90 transition-all duration-150 active:scale-[0.98] whitespace-nowrap"
          >
            <AddLinkIcon className="w-4 h-4" />
            <span>Shorten Another URL</span>
          </Link>
          {apiBaseUrl && (
            <a
              href={`${apiBaseUrl}/docs`}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center justify-center gap-2 h-10 px-4 rounded-xl bg-surface-container-low border border-outline-variant text-on-surface text-xs font-medium hover:bg-surface-container transition-all duration-150 active:scale-[0.98] whitespace-nowrap"
            >
              <InfoIcon className="w-4 h-4" />
              <span>API Docs</span>
            </a>
          )}
        </>
      }
    />
  );
}
