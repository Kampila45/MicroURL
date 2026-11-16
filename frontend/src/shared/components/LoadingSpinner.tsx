interface LoadingSpinnerProps {
  size?: "sm" | "md" | "lg";
  className?: string;
}

export function LoadingSpinner({
  size = "md",
  className = "",
}: LoadingSpinnerProps) {
  const sizeClasses: Record<"sm" | "md" | "lg", string> = {
    sm: "w-3.5 h-3.5 border-2",
    md: "w-5 h-5 border-2",
    lg: "w-6 h-6 border-2",
  };

  return (
    <span
      className={`inline-block ${sizeClasses[size]} border-t-current border-r-transparent border-b-transparent border-l-transparent rounded-full animate-spin ${className}`}
      role="status"
      aria-label="Loading"
    >
      <span className="sr-only">Loading...</span>
    </span>
  );
}
