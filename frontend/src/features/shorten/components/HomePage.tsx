import { useDocumentTitle } from "../../../shared/hooks/useDocumentTitle";
import { ResultDisplay } from "./ResultDisplay";
import { ShortenForm } from "./ShortenForm";
import { useShortenStore } from "../store/useShortenStore";

export function HomePage() {
  const status = useShortenStore((state) => state.status);
  const isSuccess = status === "success";

  useDocumentTitle(isSuccess ? "Link Ready — MicroURL" : "MicroURL");

  return (
    <main className="dot-grid flex-grow flex flex-col w-full">
      <div
        className={`w-full mx-auto px-4 sm:px-8 py-10 flex-grow flex flex-col ${
          isSuccess
            ? "max-w-[780px] gap-5 justify-start pt-8 sm:pt-12"
            : "max-w-[600px] justify-center items-center"
        }`}
      >
        {isSuccess ? (
          <>
            <div className="flex flex-col space-y-1">
              <h1 className="text-2xl sm:text-3xl font-semibold text-on-surface tracking-tight">
                Your short link is ready
              </h1>
            </div>
            <ShortenForm />
            <ResultDisplay />
          </>
        ) : (
          <div className="w-full space-y-6">
            <div className="text-center space-y-2">
              <h1 className="text-3xl sm:text-4xl font-semibold text-on-surface tracking-tight leading-tight">
                Shorten links, cleanly.
              </h1>
              <p className="text-sm text-outline">
                Custom short links, expiration controls, and instant QR codes.
              </p>
            </div>

            <ShortenForm />
          </div>
        )}
      </div>
    </main>
  );
}
