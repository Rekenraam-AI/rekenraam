export const themeNames = ['light', 'dark'] as const;
export const baseColorNames = ['olive', 'slate', 'zinc'] as const;
export const accentColorNames = ['emerald', 'blue', 'violet', 'rose', 'amber'] as const;

export type ThemeName = (typeof themeNames)[number];
export type BaseColorName = (typeof baseColorNames)[number];
export type AccentColorName = (typeof accentColorNames)[number];

/** What the owner chose: a fixed theme, or whatever the system prefers. */
export type ThemePreference = ThemeName | 'system';

type ThemeState = {
  name: ThemeName;
  preference: ThemePreference;
  initialized: boolean;
  baseColor: BaseColorName;
  accentColor: AccentColorName;
};

const storageKey = 'rekenraam-theme';
const baseColorStorageKey = 'rekenraam-base-color';
const accentColorStorageKey = 'rekenraam-accent-color';

export const themeState = $state<ThemeState>({
  name: 'light',
  preference: 'system',
  initialized: false,
  baseColor: 'olive',
  accentColor: 'emerald'
});

let mediaQuery: MediaQueryList | null = null;
let mediaQueryBound = false;

export function initializeTheme(): void {
  if (typeof window === 'undefined') {
    return;
  }

  const storedTheme = readStoredTheme();
  themeState.preference = storedTheme ?? 'system';
  syncTheme(storedTheme ?? readSystemTheme());
  syncAppearance(readStoredBaseColor() ?? 'olive', readStoredAccentColor() ?? 'emerald');
  themeState.initialized = true;

  if (mediaQueryBound) {
    return;
  }

  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
  mediaQuery.addEventListener('change', handleSystemThemeChange);
  mediaQueryBound = true;
}

export function setTheme(themeName: ThemeName): void {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(storageKey, themeName);
  }

  themeState.preference = themeName;
  syncTheme(themeName);
  themeState.initialized = true;
}

/** Forgets the stored theme so the page follows the system again. */
export function setThemePreference(preference: ThemePreference): void {
  if (preference !== 'system') {
    setTheme(preference);
    return;
  }

  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(storageKey);
  }

  themeState.preference = 'system';
  syncTheme(readSystemTheme());
  themeState.initialized = true;
}

export function setBaseColor(baseColor: BaseColorName): void {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(baseColorStorageKey, baseColor);
  }

  syncAppearance(baseColor, themeState.accentColor);
  themeState.initialized = true;
}

export function setAccentColor(accentColor: AccentColorName): void {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(accentColorStorageKey, accentColor);
  }

  syncAppearance(themeState.baseColor, accentColor);
  themeState.initialized = true;
}

export function resetAppearance(): void {
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(storageKey);
    window.localStorage.removeItem(baseColorStorageKey);
    window.localStorage.removeItem(accentColorStorageKey);
  }

  themeState.preference = 'system';
  syncTheme(readSystemTheme());
  syncAppearance('olive', 'emerald');
  themeState.initialized = true;
}

export function isThemeName(value: string | null): value is ThemeName {
  return value === 'light' || value === 'dark';
}

export function isBaseColorName(value: string | null): value is BaseColorName {
  return baseColorNames.some((name) => name === value);
}

export function isAccentColorName(value: string | null): value is AccentColorName {
  return accentColorNames.some((name) => name === value);
}

function readStoredTheme(): ThemeName | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const storedTheme = window.localStorage.getItem(storageKey);
  return isThemeName(storedTheme) ? storedTheme : null;
}

function readStoredBaseColor(): BaseColorName | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const storedBaseColor = window.localStorage.getItem(baseColorStorageKey);
  return isBaseColorName(storedBaseColor) ? storedBaseColor : null;
}

function readStoredAccentColor(): AccentColorName | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const storedAccentColor = window.localStorage.getItem(accentColorStorageKey);
  return isAccentColorName(storedAccentColor) ? storedAccentColor : null;
}

function readSystemTheme(): ThemeName {
  if (typeof window === 'undefined') {
    return 'light';
  }

  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function handleSystemThemeChange(): void {
  if (readStoredTheme() !== null) {
    return;
  }

  syncTheme(readSystemTheme());
}

function syncTheme(themeName: ThemeName): void {
  themeState.name = themeName;

  if (typeof document === 'undefined') {
    return;
  }

  document.documentElement.dataset.theme = themeName;
  document.documentElement.style.colorScheme = themeName;
}

function syncAppearance(baseColor: BaseColorName, accentColor: AccentColorName): void {
  themeState.baseColor = baseColor;
  themeState.accentColor = accentColor;

  if (typeof document === 'undefined') {
    return;
  }

  document.documentElement.dataset.baseColor = baseColor;
  document.documentElement.dataset.accentColor = accentColor;
}
