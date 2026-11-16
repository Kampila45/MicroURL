interface MicroUrlLogoProps {
  className?: string;
}

export function MicroUrlLogo({ className = "" }: MicroUrlLogoProps) {
  return (
    <span className={`inline-flex items-center gap-2.5 select-none ${className}`}>
      <span className="w-7 h-7 rounded-lg bg-on-surface text-surface flex items-center justify-center shrink-0 shadow-xs">
        <svg
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
          className="w-4 h-4"
        >
          <path
            d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"
            stroke="currentColor"
            strokeWidth="2.4"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"
            stroke="currentColor"
            strokeWidth="2.4"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
      <span className="text-base sm:text-lg font-semibold tracking-tight leading-none text-on-surface">
        Micro<span className="text-outline font-medium">URL</span>
      </span>
    </span>
  );
}
