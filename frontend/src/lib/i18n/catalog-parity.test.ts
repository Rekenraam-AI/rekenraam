import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'vitest';

const project = JSON.parse(readFileSync(new URL('../../../project.inlang/settings.json', import.meta.url), 'utf8'));
const patterns: string[] = project['plugin.inlang.messageFormat'].pathPattern;
const locales: string[] = project.locales;
const baseLocale: string = project.baseLocale;

function catalog(pattern: string, locale: string): Record<string, string> {
  const data = JSON.parse(readFileSync(new URL(`../../../${pattern.replace('{locale}', locale)}`, import.meta.url), 'utf8'));
  delete data.$schema;
  return data;
}

const keys = (data: Record<string, string>) => Object.keys(data).sort();
const parameters = (message: string) => [...new Set([...message.matchAll(/\{([A-Za-z_]\w*)\}/g)].map(match => match[1]))].sort();

// Read the compiler's configuration rather than maintaining a second list:
// newly enabled domains and locales must satisfy the same contract.
for (const pattern of patterns) {
  const base = catalog(pattern, baseLocale);
  describe(pattern, () => {
    for (const locale of locales) {
      const translated = catalog(pattern, locale);
      test(`${locale} has exactly the base-locale message keys`, () => {
        expect(keys(translated)).toEqual(keys(base));
      });
      test(`${locale} preserves nonempty messages and parameter names`, () => {
        for (const key of keys(base)) {
          const message = translated[key];
          expect(typeof message, key).toBe('string');
          expect(message.trim(), key).not.toBe('');
          expect(parameters(message), key).toEqual(parameters(base[key]));
        }
      });
    }
  });
}

for (const locale of locales) {
  test(`${locale} has no colliding keys in the shared message namespace`, () => {
    const owners = new Map<string, string>();
    for (const pattern of patterns) {
      for (const key of keys(catalog(pattern, locale))) {
        expect(owners.get(key), `${key} in ${pattern}`).toBeUndefined();
        owners.set(key, pattern);
      }
    }
  });
}
