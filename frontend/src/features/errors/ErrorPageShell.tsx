import { useEffect, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { useDocumentTitle } from "../../shared/hooks/useDocumentTitle";

interface ErrorPageShellProps {
  title: string;
  badgeText: string;
  badgeVariant: "error" | "warning";
  heading: string;
  description: ReactNode;
  actions: ReactNode;
  navigateHomeOnEnter?: boolean;
  allowReloadOnR?: boolean;
}

export function ErrorPageShell({
  title,
  badgeText,
  badgeVariant,
  heading,
  description,
  actions,
  navigateHomeOnEnter = false,
  allowReloadOnR = false,
}: ErrorPageShellProps) {
  const navigate = useNavigate();

  useDocumentTitle(title);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const tag = (event.target as HTMLElement | null)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;

      if (event.key === "Escape" || (navigateHomeOnEnter && event.key === "Enter")) {
        navigate("/");
      } else if (allowReloadOnR && (event.key === "r" || event.key === "R")) {
        window.location.reload();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [navigate, navigateHomeOnEnter, allowReloadOnR]);

  const isWarning = badgeVariant === "warning";

  return (
    <main className="dot-grid flex-grow flex flex-col w-full">
      <div className="w-full max-w-[600px] mx-auto px-4 sm:px-8 py-10 flex-grow flex flex-col justify-center">
        <div className="w-full bg-surface border border-outline-variant rounded-2xl p-6 sm:p-8 shadow-sm">
          <div
            className={`inline-flex items-center gap-2 px-3 py-1 rounded-full border text-xs font-medium mb-4 ${
              isWarning
                ? "bg-tertiary-container/15 border-tertiary/30 text-tertiary"
                : "bg-error-container/25 border-error/35 text-error"
            }`}
          >
            <span
              className={`w-1.5 h-1.5 rounded-full ${
                isWarning ? "bg-tertiary animate-pulse" : "bg-error"
              }`}
            />
            <span>{badgeText}</span>
          </div>

          <h1 className="text-2xl sm:text-3xl font-semibold text-on-surface tracking-tight leading-snug mb-2">
            {heading}
          </h1>
          <p className="text-sm text-outline mb-6 leading-relaxed">
            {description}
          </p>

          <div className="flex flex-wrap items-center gap-2.5">{actions}</div>
        </div>
      </div>
    </main>
  );
}

