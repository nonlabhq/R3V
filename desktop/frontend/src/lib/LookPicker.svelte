<script lang="ts">
  import { t } from "./i18n.svelte";
  import { errorText } from "./api";
  import { toast } from "./notify.svelte";
  import ProjectIcon from "$shared/ProjectIcon.svelte";
  import Swatches from "./Swatches.svelte";
  import { projectColor } from "$shared/palette";
  import { projectIconNames, projectIcons } from "$shared/projectIcons";
  import { appIconNames, appIcons } from "$shared/appIcons";
  import { portal } from "./portal";
  import EmojiPicker from "./EmojiPicker.svelte";
  import { emojiName, emojiOf } from "$shared/emoji";
  import { squarePicture } from "./picture";

  // How a project or the team shows, for the whole team: its icon (drawn,
  // an app's, an emoji, or its initial) on a colour, or a picture of its
  // own over them. The button shows it; a click opens the picker under it
  // (above when there's no room). Each pick is saved at once; an icon
  // picked takes the place of the picture.
  let { seed, name, icon, color, picture = "", size = 36, label, initialLabel, onlook, onpicture }: {
    seed: string; // its id (the colour picked for it when none is chosen)
    name: string; // (its initial)
    icon: string;
    color: string;
    picture?: string; // a data: URL ("" for none)
    size?: number;
    label: string; // what the button and the picker are called
    initialLabel: string; // its initial, as a choice
    onlook: (icon: string, color: string) => Promise<void>;
    onpicture: (url: string) => Promise<void>; // "" removes it
  } = $props();

  // What shows meanwhile (the pick at once; back as it was if it fails).
  // svelte-ignore state_referenced_locally
  let look = $state({ icon, color, picture });
  let busy = $state(false);
  // (the team's look as given again: after a pick, once the list is read)
  $effect(() => { look = { icon, color, picture }; });

  async function save(next: { icon: string; color: string; picture: string }, run: () => Promise<void>) {
    if (busy) return; // one at a time: the team keeps the last pick
    const was = look;
    look = next;
    busy = true;
    try {
      await run();
    } catch (e) {
      look = was;
      toast(errorText(e), "error");
    } finally {
      busy = false;
    }
  }
  const setColor = (c: string) => save({ ...look, color: c }, () => onlook(look.icon, c));
  // An icon or an emoji picked: saved (the picture gone), and the picker closes.
  async function pickIcon(i: string) {
    if (busy) return;
    close(true);
    const hadPicture = !!look.picture;
    await save({ ...look, icon: i, picture: "" }, async () => {
      if (hadPicture) await onpicture("");
      await onlook(i, look.color);
    });
  }

  const MAX_FILE = 40 << 20;
  let file = $state<HTMLInputElement>();
  async function upload(e: Event) {
    const f = (e.currentTarget as HTMLInputElement).files?.[0];
    (e.currentTarget as HTMLInputElement).value = "";
    if (!f || busy) return;
    if (f.size > MAX_FILE) {
      toast(t("This picture is too big: pick one under {size} MB.", { size: MAX_FILE >> 20 }), "error");
      return;
    }
    let url: string;
    try {
      url = await squarePicture(f);
    } catch {
      toast(t("R3V can't read this picture: try a PNG or JPEG."), "error");
      return;
    }
    close(true);
    await save({ ...look, picture: url }, () => onpicture(url));
  }

  const POP_W = 392;
  let open = $state(false);
  let btn = $state<HTMLButtonElement>();
  let popH = $state(0);
  let anchor = $state<DOMRect | null>(null);
  let popAt = $derived.by(() => {
    if (!anchor) return { left: 0, top: 0 };
    const left = Math.max(8, Math.min(anchor.left, innerWidth - POP_W - 8));
    const below = anchor.bottom + 6;
    return { left, top: below + popH > innerHeight - 8 && anchor.top - popH - 6 > 8 ? anchor.top - popH - 6 : below };
  });
  // Built-in icons or emoji: the picker opens on the kind it has.
  let tab = $state<"icons" | "emoji">("icons");
  function toggle() {
    anchor = btn?.getBoundingClientRect() ?? null;
    if (!open) tab = emojiOf(look.icon) ? "emoji" : "icons";
    open = !open;
  }
  function close(refocus = false) {
    open = false;
    if (refocus) btn?.focus();
  }
  // Esc closes the picker before the dialog it's in (the dialog listens on
  // the window too, after this).
  function onkey(e: KeyboardEvent) {
    if (open && e.key === "Escape") {
      e.preventDefault();
      close(true);
    }
  }
  function onclick(e: MouseEvent) {
    const el = e.target as HTMLElement;
    if (open && !el.closest(".look-pop") && el.closest(".look-btn") !== btn) close();
  }
</script>

<svelte:window onkeydowncapture={onkey} {onclick} onresize={() => close()} />

<button bind:this={btn} class="ghost look-btn" class:open onclick={toggle} title={label} aria-label={label} aria-haspopup="dialog" aria-expanded={open}>
  <ProjectIcon p={{ id: seed, name, status: "downloaded", icon: look.icon, color: look.color, picture: look.picture }} {size} />
</button>

{#if open}
  <div class="look-pop surface-menu" role="dialog" aria-label={label} use:portal bind:offsetHeight={popH}
    style:left="{popAt.left}px" style:top="{popAt.top}px" style:width="{POP_W}px">
    <div class="picture">
      <button class="small" onclick={() => file?.click()} disabled={busy}>{t("Upload image…")}</button>
      {#if look.picture}
        <button class="small ghost" disabled={busy} onclick={() => { close(true); save({ ...look, picture: "" }, () => onpicture("")); }}>{t("Remove image")}</button>
        <span class="faint">{t("Shown instead of the icon")}</span>
      {/if}
      <input bind:this={file} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onchange={upload} data-testid="look-file" />
    </div>
    <div class="kinds" role="tablist" aria-label={t("Icon")}>
      <button role="tab" class:on={tab === "icons"} aria-selected={tab === "icons"} onclick={() => (tab = "icons")}>{t("Icons")}</button>
      <button role="tab" class:on={tab === "emoji"} aria-selected={tab === "emoji"} onclick={() => (tab = "emoji")}>{t("Emoji")}</button>
    </div>
    {#if tab === "icons"}
      <!-- (an emoji has its own colours: the colour is for the icons) -->
      <Swatches value={projectColor(seed || name, look.color)} label={t("Colour")} disabled={busy} onpick={setColor} />
      <!-- the programs first (brand names: not translated), then the drawn icons -->
      <div class="group">
        <h4>{t("Apps")}</h4>
        <div class="icons" role="radiogroup" aria-label={t("Apps")}>
          {#each appIconNames as n (n)}
            <button class="ic" class:on={!look.picture && look.icon === n} role="radio" aria-checked={!look.picture && look.icon === n} aria-disabled={busy}
              title={appIcons[n].title} aria-label={appIcons[n].title} onclick={() => pickIcon(n)}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">{@html appIcons[n].svg}</svg>
            </button>
          {/each}
        </div>
      </div>
      <div class="icons" role="radiogroup" aria-label={t("Icon")}>
        <button class="ic" class:on={!look.picture && !look.icon} role="radio" aria-checked={!look.picture && !look.icon} aria-disabled={busy}
          title={initialLabel} aria-label={initialLabel} onclick={() => pickIcon("")}>
          {([...name.trim()][0] ?? "?").toUpperCase()}</button>
        {#each projectIconNames as n (n)}
          <button class="ic" class:on={!look.picture && look.icon === n} role="radio" aria-checked={!look.picture && look.icon === n} aria-disabled={busy}
            aria-label={n} onclick={() => pickIcon(n)}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">{@html projectIcons[n]}</svg>
          </button>
        {/each}
      </div>
    {:else}
      <EmojiPicker onpick={(e) => { const n = emojiName(e); if (n) pickIcon(n); }} />
    {/if}
    <p class="hint">{t("The whole team sees them.")}</p>
  </div>
{/if}

<style>
  .look-btn { flex: none; padding: 0; line-height: 0; border-radius: var(--radius); }
  .look-btn:hover:not(:disabled), .look-btn.open { outline: 2px solid var(--line-strong); outline-offset: 2px; background: transparent; }
  .look-pop { position: fixed; z-index: var(--z-menu); display: flex; flex-direction: column; gap: var(--sp-10);
    padding: var(--sp-12); border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .picture { display: flex; align-items: center; gap: var(--sp-8); }
  .picture .faint { font-size: var(--fs-sm); }
  .small { padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); }
  h4 { margin: 0; font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .kinds { display: flex; gap: var(--sp-2); padding: var(--sp-2); border-radius: var(--radius); background: var(--bg-sunken); align-self: flex-start; }
  .kinds button { padding: var(--sp-2) var(--sp-10); font-size: var(--fs-sm); border-color: transparent; background: transparent; color: var(--muted); }
  .kinds button.on { background: var(--panel-2); color: var(--text); }
  .group { display: flex; flex-direction: column; gap: var(--sp-6); }
  .icons { display: grid; grid-template-columns: repeat(auto-fill, 32px); gap: var(--sp-4); }
  .ic { width: 32px; height: 32px; padding: 0; display: inline-flex; align-items: center; justify-content: center;
    font-weight: var(--fw-bold); font-size: var(--fs-md); color: var(--muted); }
  .ic svg { width: 18px; height: 18px; }
  .ic.on { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
  .hint { margin: 0; font-size: var(--fs-md); color: var(--muted); }
</style>
