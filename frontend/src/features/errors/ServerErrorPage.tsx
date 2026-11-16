import { Link } from "react-router-dom";
import { ArrowBackIcon, RefreshIcon } from "../../shared/components/Icons";
import { ErrorPageShell } from "./ErrorPageShell";

export function ServerErrorPage() {
  return (
    <ErrorPageShell
      title="500 Service Unavailable — MicroURL"
      badgeText="500 · Service Unavailable"
      badgeVariant="error"
      heading="Something went wrong on our end."
      description="We could not reach the MicroURL service right now. Please try again or return to the home page."
      allowReloadOnR
      actions={
        <>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="inline-flex items-center justify-center gap-2 h-10 px-4 rounded-xl bg-on-surface text-surface text-xs font-medium hover:opacity-90 transition-all duration-150 active:scale-[0.98] whitespace-nowrap"
          >
            <RefreshIcon className="w-4 h-4" />
            <span>Try Again</span>
          </button>
          <Link
            to="/"
            className="inline-flex items-center justify-center gap-2 h-10 px-4 rounded-xl border border-outline-variant bg-surface-container-low hover:bg-surface-container text-on-surface text-xs font-medium transition-all duration-150 active:scale-[0.98] whitespace-nowrap"
          >
            <ArrowBackIcon className="w-4 h-4" />
            <span>Back to Home</span>
          </Link>
        </>
      }
    />
  );
}
