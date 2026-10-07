<script lang="ts">
  import { language, t } from "./i18n.svelte";
  import { emojiData } from "./emoji";

  // The emoji picker (emoji-picker-element: categories, a search in the
  // app's language, skin tones, the recently used), loaded when first
  // shown. onpick gets the emoji as text.
  let { onpick }: { onpick: (emoji: string) => void } = $props();

  let host = $state<HTMLDivElement>();
  let failed = $state(false);

  // Its texts, from the app's translations.
  const texts = () => ({
    categoriesLabel: t("Categories"),
    emojiUnsupportedMessage: t("This computer can't show colour emoji."),
    favoritesLabel: t("Recently used"),
    loadingMessage: t("Loading…"),
    networkErrorMessage: t("The emoji couldn't be loaded."),
    regionLabel: t("Emoji"),
    searchDescription: t("When results show, press up or down to pick one and Enter to choose it."),
    searchLabel: t("Search"),
    searchResultsLabel: t("Search results"),
    skinToneDescription: t("When open, press up or down to pick one and Enter to choose it."),
    skinToneLabel: t("Choose a skin tone (now {skinTone})", { skinTone: "{skinTone}" }),
    skinTonesLabel: t("Skin tones"),
    skinTones: [t("Default"), t("Light"), t("Medium-light"), t("Medium"), t("Medium-dark"), t("Dark")],
    categories: {
      custom: t("Custom"),
      "smileys-emotion": t("Smileys and emotions"),
      "people-body": t("People and body"),
      "animals-nature": t("Animals and nature"),
      "food-drink": t("Food and drink"),
      "travel-places": t("Travel and places"),
      activities: t("Activities"),
      objects: t("Objects"),
      symbols: t("Symbols"),
      flags: t("Flags"),
    },
  });

  $effect(() => {
    const el = host;
    if (!el) return;
    const lang = language();
    let picker: HTMLElement | undefined;
    let gone = false;
    const pick = (e: Event) => onpick((e as CustomEvent<{ unicode?: string }>).detail.unicode ?? "");
    (async () => {
      try {
        const [{ default: Picker }, data] = await Promise.all([import("emoji-picker-element/picker.js"), emojiData(lang)]);
        if (gone) return;
        picker = new Picker({ locale: data.locale, dataSource: data.url, i18n: texts() } as never) as unknown as HTMLElement;
        picker.classList.add("dark");
        picker.addEventListener("emoji-click", pick);
        el.replaceChildren(picker);
      } catch {
        failed = true;
      }
    })();
    return () => {
      gone = true;
      picker?.removeEventListener("emoji-click", pick);
      picker?.remove();
    };
  });
</script>

<div class="emoji" bind:this={host}>
  {#if failed}<p class="hint">{t("The emoji couldn't be loaded.")}</p>{/if}
</div>

<style>
  .emoji { min-height: 300px; }
  .emoji :global(emoji-picker) {
    width: 100%; height: 320px;
    --background: transparent;
    --border-color: var(--line);
    --border-size: 0;
    --button-active-background: var(--panel-2);
    --button-hover-background: var(--hover);
    --category-font-color: var(--faint);
    --indicator-color: var(--accent);
    --input-border-color: var(--line-strong);
    --input-border-radius: var(--radius);
    --input-font-color: var(--text);
    --input-placeholder-color: var(--faint);
    --outline-color: var(--accent);
    --emoji-size: 1.35rem;
    --num-columns: 9;
    --category-emoji-size: 1.1rem;
  }
  .hint { color: var(--faint); font-size: var(--fs-sm); }
</style>
