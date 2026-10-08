<script lang="ts">
  import { t } from "./i18n.svelte";
  import { Events } from "@wailsio/runtime";
  import Modal from "./Modal.svelte";
  import ProgressBar from "./ProgressBar.svelte";
  import { api, errorText, type Progress } from "./api";

  // Convert a sample, or several the same way, to another format; each new
  // file goes next to its original. Sample rate, channels and bit depth keep
  // each original's unless changed; notes say what a format forces.
  let { root, file = "", files: many, onclose, ondone }: {
    root: string;
    file?: string; // relative path of the sample
    files?: string[]; // or of several
    onclose: () => void;
    ondone: (newFiles: string[]) => void;
  } = $props();
  let files = $derived(many?.length ? many : [file]);
  let first = $derived(files[0]);
  let at = $state(0); // converting the nth (from 1)
  let failed = $state<string[]>([]);

  type Fmt = { id: string; name: string; ext: string; bitrates: number[]; rates: number[]; bits: number[] };
  let formats = $state<Fmt[]>([]);
  let format = $state("mp3");
  let kbps = $state(320);
  let rate = $state(0); // 0: same as original
  let channels = $state(0);
  let bits = $state(0);
  let info = $state<{ rate: number; channels: number; bits: number; seconds: number } | null>(null);
  let plan = $state<{ rate: number; channels: number; bits: number; bitrate: number; notes: string[] } | null>(null);
  let target = $state("");
  let busy = $state(false);
  let error = $state("");
  let progress = $state<Progress | null>(null);

  let current = $derived(formats.find((f) => f.id === format));
  const name = (p: string) => p.slice(p.lastIndexOf("/") + 1);
  const khz = (hz: number) => `${hz % 1000 ? (hz / 1000).toFixed(1) : hz / 1000} kHz`;
  const chName = (n: number) => (n === 1 ? t("mono") : n === 2 ? t("stereo") : t("{n} channels", { n }));
  // Lossy output shows its bitrate in place of the bit depth.
  const describe = (x: { rate: number; channels: number; bits: number; bitrate?: number }) =>
    [x.bitrate && `${x.bitrate} kbps`, x.rate && khz(x.rate), !x.bitrate && x.bits && `${x.bits}-bit`, x.channels && chName(x.channels)]
      .filter(Boolean).join(" · ");

  $effect(() => {
    api.ConvertFormats().then((f) => (formats = (f ?? []) as Fmt[]));
    if (files.length === 1) api.ConvertInfo(root, first).then((i) => (info = i)).catch(() => (info = null));
  });

  // Keep choices valid for the format; work out the output and the file name.
  $effect(() => {
    const f = current;
    if (!f) return;
    if (f.bitrates.length && !f.bitrates.includes(kbps)) kbps = f.bitrates[0];
    if (rate && !f.rates.includes(rate)) rate = 0;
    if (bits && !f.bits.includes(bits)) bits = 0;
    const [k, r, c, b] = [f.bitrates.length ? kbps : 0, rate, channels, bits];
    api.ConvertTarget(root, first, f.id).then((t) => (target = t)).catch(() => (target = ""));
    api.ConvertPlan(root, first, f.id, k, r, c, b).then((p) => { plan = p; error = ""; })
      .catch((e) => { plan = null; error = errorText(e); });
  });

  $effect(() => Events.On("progress", (ev: { data: Progress }) => {
    if (busy && ev.data.root === root) progress = ev.data.stage === "done" ? null : ev.data;
  }));

  // One after another; one that fails is said, the rest go on.
  async function run() {
    busy = true;
    error = "";
    failed = [];
    const made: string[] = [];
    for (const [i, f] of files.entries()) {
      at = i + 1;
      progress = null;
      try {
        made.push(await api.ConvertFile(root, f, format, current?.bitrates.length ? kbps : 0, rate, channels, bits));
      } catch (e) {
        failed = [...failed, `${name(f)}: ${errorText(e)}`];
      }
    }
    busy = false;
    if (!failed.length) ondone(made);
    else if (made.length) error = t("Converted {done} of {total}; these weren't:", { done: made.length, total: files.length });
  }
</script>

<Modal title={files.length > 1 ? t("Convert {n} samples", { n: files.length }) : t("Convert {file}", { file: name(first) })}
  onclose={() => { if (!busy) onclose(); }}>
  {#if info}<p class="faint small orig">{t("Original:")} {describe(info)}{info.seconds ? ` · ${info.seconds.toFixed(1)} s` : ""}</p>{/if}

  <div class="grid">
    <label for="cf">{t("Format")}</label>
    <select id="cf" bind:value={format} disabled={busy}>
      {#each formats as f (f.id)}<option value={f.id}>{f.name}</option>{/each}
    </select>

    {#if current?.bitrates.length}
      <label for="cb">{t("Quality")}</label>
      <select id="cb" bind:value={kbps} disabled={busy}>
        {#each current.bitrates as b}<option value={b}>{b} kbps{b === current.bitrates[0] ? ` (${t("best")})` : ""}</option>{/each}
      </select>
    {/if}

    {#if current}
      <label for="cr">{t("Sample rate")}</label>
      <select id="cr" bind:value={rate} disabled={busy}>
        <option value={0}>{t("Same as original")}{info?.rate ? ` (${khz(info.rate)})` : ""}</option>
        {#each current.rates as r}<option value={r}>{khz(r)}</option>{/each}
      </select>

      <label for="cc">{t("Channels")}</label>
      <select id="cc" bind:value={channels} disabled={busy}>
        <option value={0}>{t("Same as original")}{info?.channels ? ` (${chName(info.channels)})` : ""}</option>
        <option value={2}>{t("Stereo")}</option>
        <option value={1}>{t("Mono (channels mixed)")}</option>
      </select>

      {#if current.bits.length}
        <label for="cd">{t("Bit depth")}</label>
        <select id="cd" bind:value={bits} disabled={busy}>
          <option value={0}>{t("Same as original")}{info?.bits ? ` (${info.bits}-bit)` : ""}</option>
          {#each current.bits as b}<option value={b}>{b}-bit</option>{/each}
        </select>
      {/if}
    {/if}
  </div>

  {#if plan}
    {#if files.length > 1}
      <p class="small result">{t("Writes each as {what} next to its original; “same as original” keeps each one's own. The originals stay.", { what: describe(plan) })}</p>
    {:else}
    <p class="small result">{target ? t("Writes {what} as {file} next to the original. The original stays.", { what: describe(plan), file: name(target) })
      : t("Writes {what} next to the original. The original stays.", { what: describe(plan) })}</p>
    {/if}
    {#each plan.notes as n}<p class="small note">{n}</p>{/each}
  {/if}
  {#if busy}
    {#if files.length > 1}<p class="small result">{t("{n} of {total}: {file}", { n: at, total: files.length, file: name(files[at - 1] ?? "") })}</p>{/if}
    <ProgressBar p={progress} waiting={t("Starting…")} />
  {/if}
  {#if error}<p class="error small">{error}</p>{/if}
  {#each failed as f}<p class="error small">{f}</p>{/each}

  {#snippet footer()}
    <button onclick={onclose} disabled={busy}>{t("Cancel")}</button>
    <button class="primary" onclick={run} disabled={busy || !current || !plan}>{busy ? t("Converting…") : t("Convert")}</button>
  {/snippet}
</Modal>

<style>
  .orig { margin: 0 0 var(--sp-12); }
  .grid { display: grid; grid-template-columns: auto 1fr; gap: var(--sp-10) var(--sp-14); align-items: center; margin-bottom: var(--sp-10); }
  .grid label { margin: 0; }
  select { width: 100%; }
  .small { font-size: var(--fs-md); }
  .result { color: var(--muted); margin: var(--sp-10) 0 var(--sp-4); }
  .note { color: var(--warn); margin: var(--sp-2) 0; }
  .error { color: var(--danger); }
</style>
