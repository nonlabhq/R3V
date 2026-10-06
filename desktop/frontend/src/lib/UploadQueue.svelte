<script lang="ts">
  import { t } from "./i18n.svelte";
  import { formatBytes } from "./api";
  import Modal from "./Modal.svelte";
  import ProgressBar from "./ProgressBar.svelte";
  import FileIcon from "./FileIcon.svelte";
  import { preuploads, transfers } from "./preupload.svelte";

  // The upload queue: each project's commit or update under way, and the big
  // files going up in the background, with the ones waiting after them.
  let { names, onclose }: { names: Record<string, string>; onclose: () => void } = $props();

  const name = (root: string) => names[root] || root.slice(Math.max(root.lastIndexOf("/"), root.lastIndexOf("\\")) + 1);
  const file = (p: string) => p.slice(p.lastIndexOf("/") + 1);
  const dir = (p: string) => p.slice(0, p.lastIndexOf("/") + 1);
  const audio = /\.(wav|aiff?|mp3|flac|ogg|m4a)$/i;
  const kind = (p: string) => (/\.als$/i.test(p) ? "set" : audio.test(p) ? "audio" : "other");
  let steps = $derived(Object.values(transfers));
  let early = $derived(Object.values(preuploads));
</script>

<Modal title={t("Uploads")} {onclose} width={560}>
  {#if !steps.length && !early.length}
    <p class="muted">{t("Nothing is going up right now.")}</p>
  {/if}

  {#each steps as s (s.root)}
    <section>
      <h3>{name(s.root)}</h3>
      <ProgressBar p={s} />
      {#if s.speed}<p class="faint small">{formatBytes(s.speed)}/s</p>{/if}
    </section>
  {/each}

  {#each early as p (p.root)}
    {@const pct = p.total ? Math.round((100 * p.bytes) / p.total) : 0}
    <section>
      <h3>{name(p.root)} <span class="faint">· {t("in the background")}</span></h3>
      <div class="row now">
        <FileIcon kind={kind(p.path)} />
        <div class="what">
          <div class="fname" title={p.path}>{file(p.path)}{#if dir(p.path)}<span class="fdir"> {dir(p.path)}</span>{/if}</div>
          <div class="bar"><div style:width="{pct}%"></div></div>
          <div class="faint small">{formatBytes(p.bytes)} / {formatBytes(p.total)} · {pct}%{#if p.speed} · {formatBytes(p.speed)}/s{/if}</div>
        </div>
      </div>
      {#each p.waiting as w (w.path)}
        <div class="row">
          <FileIcon kind={kind(w.path)} />
          <div class="what"><div class="fname" title={w.path}>{file(w.path)}{#if dir(w.path)}<span class="fdir"> {dir(w.path)}</span>{/if}</div></div>
          <span class="faint small">{formatBytes(w.size)} · {t("waiting")}</span>
        </div>
      {/each}
    </section>
  {/each}

  <p class="faint small note">{t("Big files go up in the background once they stop changing, so committing them later takes a moment. They stop if the file is deleted, changed or left out.")}</p>

  {#snippet footer()}
    <button onclick={onclose}>{t("Close")}</button>
  {/snippet}
</Modal>

<style>
  section { padding: var(--sp-10) 0; border-top: var(--border-width) solid var(--line); }
  section:first-child { border-top: none; padding-top: 0; }
  h3 { margin: 0 0 var(--sp-8); font-size: var(--fs-md); font-weight: var(--fw-semibold); }
  .row { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-6) 0; }
  .what { flex: 1; min-width: 0; }
  .fname { font-size: var(--fs-md); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fdir { color: var(--faint); font-size: var(--fs-sm); }
  .bar { height: 4px; margin: var(--sp-4) 0; border-radius: var(--radius-pill); background: var(--line); overflow: hidden; }
  .bar div { height: 100%; background: var(--ok); transition: width .3s; }
  .small { font-size: var(--fs-sm); margin: 0; }
  .note { margin-top: var(--sp-10); }
</style>
