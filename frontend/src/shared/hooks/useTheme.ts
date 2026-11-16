import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";

function applyThemeToDocument(theme: Theme): void {
  const root = document.documentElement;
  root.setAttribute("data-theme", theme);
  if (theme === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
  localStorage.setItem("theme", theme);
}

function readInitialTheme(): Theme {
  if (typeof window === "undefined") {
    return "dark";
  }
  const saved = localStorage.getItem("theme");
  if (saved === "light" || saved === "dark") {
    return saved;
  }
  return "dark";
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(readInitialTheme);

  useEffect(() => {
    applyThemeToDocument(theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === "light" ? "dark" : "light"));
  }, []);

  return { theme, toggleTheme };
}
