import { Link, useNavigate } from "react-router-dom";
import { AddLinkIcon, ArrowBackIcon } from "../../shared/components/Icons";
import { ErrorPageShell } from "./ErrorPageShell";

export function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <ErrorPageShell
      title="404 Not Found — MicroURL"
      badgeText="404 · Link Not Found"
      badgeVariant="error"
      heading="Short link not found or removed."
      description="The short link or page you requested does not exist or is no longer available."
      navigateHomeOnEnter
      actions={
        <>
          <Link
            to="/"
            className="h-10 px-4 rounded-xl bg-on-surface text-surface text-xs font-medium inline-flex items-center justify-center gap-2 hover:opacity-90 active:scale-[0.98] transition-all duration-150 whitespace-nowrap"
          >
            <AddLinkIcon className="w-4 h-4" />
            <span>Shorten a new link</span>
          </Link>
          <button
            type="button"
            onClick={() => {
              if (window.history.length > 1) {
                navigate(-1);
              } else {
                navigate("/");
              }
            }}
            className="h-10 px-4 rounded-xl bg-surface-container-low border border-outline-variant text-on-surface text-xs font-medium inline-flex items-center justify-center gap-2 hover:bg-surface-container active:scale-[0.98] transition-all duration-150 whitespace-nowrap"
          >
            <ArrowBackIcon className="w-4 h-4 text-outline" />
            <span>Back to Safety</span>
          </button>
        </>
      }
    />
  );
}
