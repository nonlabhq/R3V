<script lang="ts" module>
  // A shortcut at the end of a tooltip ("New tab (Ctrl+T)") is shown as keys.
  const SHORTCUT = /^([\s\S]*?)\s*\(((?:Ctrl|Shift|Alt|Cmd|F\d{1,2}|Esc)\b[^()]*)\)$/;
  export function splitTip(text: string): { text: string; keys: string } {
    const m = text.match(SHORTCUT);
    return m ? { text: m[1], keys: m[2] } : { text, keys: "" };
  }
</script>

<script lang="ts">
  // The app's tooltips, one look for all: an element's title is shown here
  // instead of the browser's (it moves to data-tip, so the browser's doesn't
  // show too; an element named only by it keeps it as its aria-label). After
  // a moment on the element, or at once when going from one to the next.
  // Mounted once (App).
  const DELAY = 450, WARM = 300;
  let tip = $state<{ text: string; keys: string; x: number; y: number; below: boolean } | null>(null);
  let height = $state(0), width = $state(0);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let lastHidden = -Infinity;
  let on: HTMLElement | null = null;

  function tipOf(el: HTMLElement): string {
    const title = el.getAttribute("title");
    if (title) {
      el.setAttribute("data-tip", title);
      el.removeAttribute("title");
      if (!el.getAttribute("aria-label") && !el.textContent?.trim()) el.setAttribute("aria-label", title);
    }
    return el.getAttribute("data-tip") ?? "";
  }
  function show(el: HTMLElement) {
    const text = tipOf(el);
    if (!text) return;
    clearTimeout(timer);
    on = el;
    const open = () => {
      if (on !== el || !el.isConnected) return;
      const r = el.getBoundingClientRect();
      const below = r.bottom + 8 + 40 < window.innerHeight;
      tip = { ...splitTip(text), x: r.left + r.width / 2, y: below ? r.bottom + 6 : r.top - 6, below };
    };
    if (tip || performance.now() - lastHidden < WARM) open();
    else timer = setTimeout(open, DELAY);
  }
  function hide() {
    clearTimeout(timer);
    if (tip) lastHidden = performance.now();
    tip = null;
    on = null;
  }
  const target = (e: Event) => (e.target as HTMLElement | null)?.closest?.<HTMLElement>("[title], [data-tip]") ?? null;

  function onpointerover(e: PointerEvent) {
    if (e.pointerType === "touch") return;
    const el = target(e);
    if (el === on) return;
    if (!el) return hide();
    hide();
    show(el);
  }
  function onpointerout(e: PointerEvent) {
    if (on && !(e.relatedTarget instanceof Node && on.contains(e.relatedTarget))) hide();
  }
  function onfocusin(e: FocusEvent) {
    const el = target(e);
    if (el && (e.target as HTMLElement).matches?.(":focus-visible")) show(el);
  }
  // Where it goes: centred on the element, inside the window.
  let left = $derived(tip ? Math.min(Math.max(8, tip.x - width / 2), window.innerWidth - 8 - width) : 0);
  let top = $derived(tip ? (tip.below ? tip.y : tip.y - height) : 0);
</script>

<svelte:document {onpointerover} {onpointerout} {onfocusin} onfocusout={hide} onpointerdown={hide} onkeydown={hide} />
<svelte:window onscroll={hide} onwheel={hide} onblur={hide} />

{#if tip}
  <div class="tip" role="tooltip" bind:clientHeight={height} bind:clientWidth={width}
    style:left="{left}px" style:top="{top}px">
    {#if tip.text}<span class="text">{tip.text}</span>{/if}
    {#if tip.keys}<kbd>{tip.keys}</kbd>{/if}
  </div>
{/if}

<style>
  .tip { position: fixed; z-index: var(--z-blocking); pointer-events: none; max-width: 300px; display: flex; align-items: center;
    gap: var(--sp-8); padding: var(--sp-4) var(--sp-8); border: var(--border-width) solid var(--line-strong); border-radius: var(--radius);
    background: var(--panel-3); box-shadow: var(--shadow-pop); color: var(--text); font-size: var(--fs-sm); line-height: 1.35;
    animation: in .12s ease-out; }
  .text { white-space: pre-line; overflow-wrap: anywhere; }
  kbd { flex: none; font-family: var(--font-mono); font-size: var(--fs-xs); padding: 0 var(--sp-4); border: var(--border-width) solid var(--line);
    border-radius: var(--radius-sm); background: var(--panel-2); color: var(--muted); white-space: nowrap; }
  @keyframes in { from { opacity: 0; transform: translateY(2px); } }
</style>
