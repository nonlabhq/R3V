// The emoji picker's data (the App's: names in its languages); the
// emoji themselves are $shared/emoji.

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
