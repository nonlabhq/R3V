// An emoji as a project's icon: stored as its code points in hex ("e-1f3b5"
// for 🎵), so it is a name like the others (a-z, 0-9, "-": what a team
// keeps, remote.ValidLookName). A build without emoji shows the initial.

const STORED = /^e(-[0-9a-f]{1,6})+$/;

/** The stored name of an emoji ("" when it is too long to keep). */
export function emojiName(emoji: string): string {
  const name = "e-" + [...emoji].map((c) => c.codePointAt(0)!.toString(16)).join("-");
  return name.length <= 64 ? name : "";
}

/** The emoji a stored name stands for, null when it isn't one. */
export function emojiOf(name: string | undefined): string | null {
  if (!name || !STORED.test(name)) return null;
  const points = name.slice(2).split("-").map((h) => parseInt(h, 16));
  if (points.some((p) => p > 0x10ffff)) return null;
  return String.fromCodePoint(...points);
}

// The emoji data for each language of the app (CLDR names, so a search
// works in it), kept in the app: nothing is fetched from elsewhere.
const data: Record<string, () => Promise<{ default: string }>> = {
  en: () => import("emoji-picker-element-data/en/cldr/data.json?url"),
  de: () => import("emoji-picker-element-data/de/cldr/data.json?url"),
  es: () => import("emoji-picker-element-data/es/cldr/data.json?url"),
  fr: () => import("emoji-picker-element-data/fr/cldr/data.json?url"),
  it: () => import("emoji-picker-element-data/it/cldr/data.json?url"),
  ja: () => import("emoji-picker-element-data/ja/cldr/data.json?url"),
  ko: () => import("emoji-picker-element-data/ko/cldr/data.json?url"),
  "pt-BR": () => import("emoji-picker-element-data/pt/cldr/data.json?url"),
  "zh-CN": () => import("emoji-picker-element-data/zh/cldr/data.json?url"),
  "zh-TW": () => import("emoji-picker-element-data/zh-hant/cldr/data.json?url"),
};

/** Where the emoji data for language lang is (English for one without). */
export async function emojiData(lang: string): Promise<{ locale: string; url: string }> {
  const locale = lang in data ? lang : "en";
  return { locale, url: (await data[locale]()).default };
}
