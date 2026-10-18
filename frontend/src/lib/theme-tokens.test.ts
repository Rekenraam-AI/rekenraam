import { readFileSync, readdirSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Every colour utility in the frontend names a semantic token from app.css
 * (#324, found by the #278 audit; extends the #259 badge check to the whole pattern).
 *
 * Tailwind emits nothing for a colour it does not know, so a utility such as
 * `text-destructive` or `text-warning-foreground` compiles silently and leaves
 * the element in its inherited colour. That is how 22 error messages and
 * negative gains lost their danger colour, and how the warning and danger
 * confirmation buttons were drawn in foreground text on their own fill — under
 * 2:1 in the dark theme — without any build or type check noticing. Palette
 * colours (`text-green-700`, `bg-black/50`) bypass the theme the same way.
 */

const src = fileURLToPath(new URL('..', import.meta.url));
const css = readFileSync(path.join(src, 'app.css'), 'utf8');
const themeBlock = /@theme inline\s*\{([^}]*)\}/.exec(css)?.[1] ?? '';
const tokens = new Set([...themeBlock.matchAll(/--color-([a-z0-9-]+):/g)].map(([, name]) => name));

const keywords = new Set(['inherit', 'current', 'transparent']);
// Utility prefixes whose value is a colour. Each also has non-colour values
// (sizes, alignment, widths, styles), which the pattern below leaves out.
const colourUtility =
  /(?<![\w/.-])(?:[a-z0-9[\]=&-]+:)*(text|bg|border(?:-[trblxy])?|ring(?:-offset)?|outline|fill|stroke|divide|decoration|accent|caret|from|via|to|placeholder|shadow)-([a-z][a-z0-9-]*)(?:\/\d+)?(?![\w-])/g;
const nonColour =
  /^(xs|sm|base|md|lg|xl|\d?xl|left|right|center|justify|start|end|wrap|nowrap|balance|pretty|ellipsis|clip|solid|dashed|dotted|double|none|collapse|separate|x|y|t|b|l|r|s|e|fixed|local|scroll|cover|contain|auto|top|bottom|no-repeat|repeat|origin|offset|inset|underline|thin|thick|slice|clone|linear|radial|conic|anchor|width|linejoin|linecap|dasharray|opacity|ellipsis|px|panel)(-.*)?$/;

function sources(directory: string): string[] {
  return readdirSync(directory).flatMap((name) => {
    const file = path.join(directory, name);
    if (statSync(file).isDirectory()) return name === 'paraglide' ? [] : sources(file);
    return /\.(svelte|ts)$/.test(name) && !/\.(test|d)\.ts$/.test(name) ? [file] : [];
  });
}

describe('theme tokens', () => {
  it('reads the theme tokens from app.css', () => {
    expect(tokens.has('danger')).toBe(true);
    expect(tokens.has('surface-strong')).toBe(true);
  });

  it('uses only colour utilities that name a semantic token', () => {
    const unknown: string[] = [];
    for (const file of sources(src)) {
      const text = readFileSync(file, 'utf8');
      for (const match of text.matchAll(colourUtility)) {
        const [utility, , value] = match;
        if (tokens.has(value) || keywords.has(value) || nonColour.test(value)) continue;
        const line = text.slice(0, match.index).split('\n').length;
        unknown.push(`${path.relative(src, file)}:${line} ${utility}`);
      }
    }
    expect(unknown).toEqual([]);
  });

  // Fading a warning fill to 90% opacity over the surface reads 4.2:1 in the
  // light theme (#341); warning buttons show hover without changing colour.
  it('never fades a warning-filled control on hover', () => {
    const faded: string[] = [];
    for (const file of sources(src)) {
      readFileSync(file, 'utf8').split('\n').forEach((line, index) => {
        if (/(?<![\w-])bg-warning(?![\w/-])/.test(line) && /hover:opacity-/.test(line)) faded.push(`${path.relative(src, file)}:${index + 1}`);
      });
    }
    expect(faded).toEqual([]);
  });
});
