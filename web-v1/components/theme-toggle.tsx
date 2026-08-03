"use client";

import { Moon, Sun } from "lucide-react";
import { useSyncExternalStore } from "react";
import { useTheme } from "next-themes";

// ThemeToggle 供应用外壳调用，切换当前的明暗主题。
export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );

  if (!mounted) {
    return <span className="block size-8" aria-hidden />;
  }

  const isDark = resolvedTheme === "dark";
  return (
    <button
      aria-label={isDark ? "切换为浅色主题" : "切换为深色主题"}
      className="ui-icon-button"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      title={isDark ? "浅色主题" : "深色主题"}
      type="button"
    >
      {isDark ? <Sun className="size-4" /> : <Moon className="size-4" />}
    </button>
  );
}
