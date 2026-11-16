import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from "react";

interface TurnstileRenderOptions {
  sitekey: string;
  theme?: "light" | "dark" | "auto";
  callback?: (token: string) => void;
  "error-callback"?: () => void;
  "expired-callback"?: () => void;
}

interface TurnstileApi {
  render: (
    container: HTMLElement,
    options: TurnstileRenderOptions
  ) => string;
  reset: (widgetId?: string) => void;
  remove: (widgetId?: string) => void;
}

declare global {
  interface Window {
    turnstile?: TurnstileApi;
  }
}

export interface CaptchaFieldRef {
  reset: () => void;
}

interface CaptchaFieldProps {
  onToken: (token: string) => void;
  onError?: () => void;
  onExpire?: () => void;
  theme?: "light" | "dark";
}

const TURNSTILE_SCRIPT_ID = "cf-turnstile-script";
const TURNSTILE_SCRIPT_SRC =
  "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

export const CaptchaField = forwardRef<CaptchaFieldRef, CaptchaFieldProps>(
  ({ onToken, onError, onExpire, theme = "dark" }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const widgetIdRef = useRef<string | null>(null);
    const onTokenRef = useRef(onToken);
    const onErrorRef = useRef(onError);
    const onExpireRef = useRef(onExpire);

    useEffect(() => {
      onTokenRef.current = onToken;
      onErrorRef.current = onError;
      onExpireRef.current = onExpire;
    }, [onToken, onError, onExpire]);

    const siteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY || "";

    useImperativeHandle(ref, () => ({
      reset: () => {
        if (widgetIdRef.current && window.turnstile) {
          window.turnstile.reset(widgetIdRef.current);
        }
      },
    }));

    useEffect(() => {
      if (!siteKey || !containerRef.current) {
        return;
      }

      let cancelled = false;

      const renderWidget = () => {
        if (cancelled || !containerRef.current || !window.turnstile) {
          return;
        }
        if (widgetIdRef.current) {
          window.turnstile.remove(widgetIdRef.current);
          widgetIdRef.current = null;
        }
        widgetIdRef.current = window.turnstile.render(containerRef.current, {
          sitekey: siteKey,
          theme,
          callback: (token: string) => onTokenRef.current(token),
          "error-callback": () => onErrorRef.current?.(),
          "expired-callback": () => {
            onExpireRef.current?.();
            if (widgetIdRef.current && window.turnstile) {
              window.turnstile.reset(widgetIdRef.current);
            }
          },
        });
      };

      if (window.turnstile) {
        renderWidget();
      } else {
        let script = document.getElementById(
          TURNSTILE_SCRIPT_ID
        ) as HTMLScriptElement | null;
        if (!script) {
          script = document.createElement("script");
          script.id = TURNSTILE_SCRIPT_ID;
          script.src = TURNSTILE_SCRIPT_SRC;
          script.async = true;
          script.defer = true;
          document.head.appendChild(script);
        }
        script.addEventListener("load", renderWidget);
        return () => {
          cancelled = true;
          script?.removeEventListener("load", renderWidget);
          if (widgetIdRef.current && window.turnstile) {
            window.turnstile.remove(widgetIdRef.current);
            widgetIdRef.current = null;
          }
        };
      }

      return () => {
        cancelled = true;
        if (widgetIdRef.current && window.turnstile) {
          window.turnstile.remove(widgetIdRef.current);
          widgetIdRef.current = null;
        }
      };
    }, [siteKey, theme]);

    if (!siteKey) {
      return (
        <div className="flex items-center justify-between rounded-xl bg-surface-container-low border border-outline-variant px-3.5 py-2 text-xs">
          <div className="flex items-center gap-2 text-outline truncate">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" />
            <span className="text-on-surface-variant font-medium">
              Human check
            </span>
            <span className="truncate">
              bypassed in local development
            </span>
          </div>
          <span className="text-[11px] px-2 py-0.5 rounded-full bg-surface border border-outline-variant text-outline shrink-0 ml-2">
            Dev
          </span>
        </div>
      );
    }

    return (
      <div className="flex items-center justify-between rounded-xl bg-surface-container-low border border-outline-variant px-3.5 py-2.5">
        <div ref={containerRef} data-testid="turnstile-widget" />
      </div>
    );
  }
);

CaptchaField.displayName = "CaptchaField";
