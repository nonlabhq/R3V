<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, errorText } from "./api";
  import type { Overview, TrackSummary } from "../../bindings/github.com/nonlabhq/r3v/internal/als/models";

  // One side of a track both sides changed: the track and its neighbours as
  // in Live's Arrangement, in short, the track's clips in the side's colour.
  // Read as SetView reads a set (SetOverview), one version, nothing compared.
  let { root, file, version, track, color, name = "" }: {
    root: string; file: string;
    version: string; // a version id, "" the project folder now
    track: string;   // the track's id in the set
    color: string;   // CSS colour of the side
    name?: string;   // the track's name, when the side has no such track
  } = $props();

  let set = $state<Overview | null>(null);
  let err = $state("");
  let gen = 0;
  $effect(() => {
    const g = ++gen;
    set = null;
    err = "";
    api.SetOverview(root, file, version, "", "none")
      .then((d) => { if (g === gen) set = d?.now ?? null; })
      .catch((e) => { if (g === gen) err = errorText(e); });
  });

  // The track, with up to three tracks around it (no returns).
  const AROUND = 4;
  let rows = $derived.by(() => {
    if (!set) return [];
    const tracks = set.tracks.filter((x) => x.kind !== "return" || x.id === track);
    const at = tracks.findIndex((x) => x.id === track);
    if (at < 0) return tracks.slice(0, AROUND - 1);
    const from = Math.max(0, Math.min(at - 1, tracks.length - AROUND));
    return tracks.slice(from, from + AROUND);
  });
  let has = $derived(!!set?.tracks.some((x) => x.id === track));
  let total = $derived(Math.max(4, set?.length ?? 0, ...rows.flatMap((r) => arr(r).map((c) => c.end))));
  const arr = (x: TrackSummary) => x.clips.filter((c) => c.slot < 0);
  const pct = (beats: number) => `${(beats / total) * 100}%`;
</script>

<div class="lanes" style:--side={color}>
  {#if err}
    <p class="muted small">{t("Couldn't read the set:")} {err}</p>
  {:else if !set}
    <p class="muted small">{t("Reading the set…")}</p>
  {:else}
    {#if !has}
      <div class="row target gone">
        <span class="name">{name}</span>
        <span class="lane"><span class="note">{t("Not in this version")}</span></span>
      </div>
    {/if}
    {#each rows as r (r.id)}
      {@const mine = r.id === track}
      {@const clips = arr(r)}
      <div class="row" class:target={mine} class:off={r.muted}>
        <span class="name" title={r.name}>{r.name}</span>
        <span class="lane">
          {#each clips as c}
            <span class="clip" style:left={pct(c.start)} style:width={pct(c.end - c.start)} title={c.name}></span>
          {/each}
          {#if mine && !clips.length}
            <span class="note">{r.clips.length ? tn(r.clips.length, "{n} clip in session", "{n} clips in session") : t("no clips")}</span>
          {/if}
        </span>
      </div>
    {/each}
  {/if}
</div>

<style>
  .lanes { display: flex; flex-direction: column; gap: var(--sp-6); min-height: 0; }
  .row { display: flex; align-items: center; gap: var(--sp-10); height: 30px; }
  .name { flex: 0 0 72px; font-size: var(--fs-sm); color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .target .name { color: var(--text); font-weight: var(--fw-bold); }
  .lane { flex: 1; position: relative; height: 100%; background: var(--line-soft); border-radius: var(--radius-sm); }
  .clip { position: absolute; top: 3px; bottom: 3px; min-width: 2px; border-radius: var(--radius-xs); background: var(--hover-strong); }
  .target .clip { background: var(--side); }
  .off .clip { opacity: .5; }
  .note { position: absolute; inset: 0; display: flex; align-items: center; padding: 0 var(--sp-8); font-size: var(--fs-xs); color: var(--faint); }
  .gone .lane { background: repeating-linear-gradient(135deg, transparent 0 6px, var(--line-soft) 6px 12px); }
  .small { font-size: var(--fs-sm); margin: 0; }
</style>
