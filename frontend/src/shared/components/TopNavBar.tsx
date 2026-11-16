import { Link } from "react-router-dom";
import { useTheme } from "../hooks/useTheme";
import { BarChartIcon, MoonIcon, SunIcon } from "./Icons";
import { MicroUrlLogo } from "./MicroUrlLogo";

interface TopNavBarProps {
  onResetHome?: () => void;
}

export function TopNavBar({ onResetHome }: TopNavBarProps) {
  const { theme, toggleTheme } = useTheme();

  return (
    <header className="w-full border-b border-outline-variant bg-background/85 backdrop-blur-md sticky top-0 z-50">
      <div className="flex justify-between items-center w-full max-w-[820px] mx-auto px-4 sm:px-8 h-14">
        <Link
          to="/"
          onClick={onResetHome}
          aria-label="MicroURL home"
          className="flex items-center group transition-opacity hover:opacity-85"
        >
          <MicroUrlLogo />
        </Link>

        <div className="flex items-center gap-1">
          <Link
            to="/stats"
            className="p-2 rounded-lg text-outline hover:text-on-surface hover:bg-surface-container transition-colors duration-150"
            title="Link Stats"
            aria-label="Open Link Stats"
          >
            <BarChartIcon className="w-[18px] h-[18px]" />
          </Link>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
            title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
            className="p-2 rounded-lg text-outline hover:text-on-surface hover:bg-surface-container transition-colors duration-150 active:scale-[0.98]"
          >
            {theme === "dark" ? (
              <SunIcon className="w-[18px] h-[18px]" />
            ) : (
              <MoonIcon className="w-[18px] h-[18px]" />
            )}
          </button>
        </div>
      </div>
    </header>
  );
}
