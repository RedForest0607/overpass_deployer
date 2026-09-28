'use client';
import { useEffect, useState } from 'react';

type Theme = 'light' | 'dark';
const storageKey = 'overpass-studio-theme';

export function useTheme() {
  const [theme, setTheme] = useState<Theme>('light');
  useEffect(() => {
    const system = window.matchMedia('(prefers-color-scheme: dark)');
    let preference: string | null = null;
    try {
      preference = localStorage.getItem(storageKey);
    } catch {
      /* Storage may be disabled. */
    }
    const apply = (value: Theme) => {
      setTheme(value);
      document.documentElement.classList.toggle('dark', value === 'dark');
      document.documentElement.style.colorScheme = value;
    };
    apply(
      preference === 'dark' || preference === 'light'
        ? preference
        : system.matches
          ? 'dark'
          : 'light',
    );
    const onSystem = () => {
      if (!preference) apply(system.matches ? 'dark' : 'light');
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key !== storageKey) return;
      preference = event.newValue;
      apply(
        preference === 'dark' || preference === 'light'
          ? preference
          : system.matches
            ? 'dark'
            : 'light',
      );
    };
    const onChoice = (event: Event) => {
      preference = (event as CustomEvent<Theme>).detail;
      apply(preference as Theme);
    };
    system.addEventListener('change', onSystem);
    window.addEventListener('storage', onStorage);
    window.addEventListener('studio-theme', onChoice);
    return () => {
      system.removeEventListener('change', onSystem);
      window.removeEventListener('storage', onStorage);
      window.removeEventListener('studio-theme', onChoice);
    };
  }, []);
  function toggleTheme() {
    const next = theme === 'dark' ? 'light' : 'dark';
    try {
      localStorage.setItem(storageKey, next);
    } catch {
      /* Keep the in-memory preference. */
    }
    window.dispatchEvent(new CustomEvent('studio-theme', { detail: next }));
  }
  return { theme, toggleTheme };
}
