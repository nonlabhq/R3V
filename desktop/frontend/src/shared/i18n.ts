// Texts in the shared components: English is the key, translated by
// whoever uses them (the App, R3V Cloud's web app) through setTranslator.
// Until one is set, the English text with its {vars} filled in.

type Vars = Record<string, string | number>;
type Translate = (text: string, vars?: Vars) => string;
type TranslateN = (n: number, one: string, other: string, vars?: Vars) => string;

function fill(text: string, vars?: Vars): string {
  if (!vars) return text;
  return text.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m));
}

let tr: Translate = fill;
let trn: TranslateN = (n, one, other, vars) => fill(n === 1 ? one : other, { n, ...vars });
let lang = () => (typeof navigator === "undefined" ? "en" : navigator.language || "en");

/** Sets the translator the shared components use (a reactive one keeps
 * them in the language in use), and the language (for dates). */
export function setTranslator(t: Translate, tn: TranslateN, language?: () => string) {
  tr = t;
  trn = tn;
  if (language) lang = language;
}

/** The language in use ("zh-TW"). */
export const language = () => lang();

/** A text in the language in use. */
export const t: Translate = (text, vars) => tr(text, vars);

/** One of two texts by count: tn(n, "{n} file", "{n} files"); n is in vars. */
export const tn: TranslateN = (n, one, other, vars) => trn(n, one, other, vars);
