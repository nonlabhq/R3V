<script lang="ts">
  import { t } from "../i18n";
  import { ago, errorText, formatBytes } from "../text";
  import type { TeamSource, TeamUsageData as TeamUsageView } from "./source";

  // A team's Usage tab (docs/design/team-settings.md): how much it keeps,
  // by project. On its own storage R3V counts on request (every version's
  // folder lists read once), the count kept with its time; on R3V Cloud,
  // the service's numbers and the plan's.
  let { source }: { source: TeamSource } = $props();

  let v = $state<TeamUsageView | null>(null);
  let error = $state("");
  let counting = $state(false);
  $effect(() => {
    const src = source;
    v = null;
    error = "";
    src.usage().then((u) => { if (source === src) v = u; }).catch((e) => { if (source === src) error = errorText(e); });
  });
  async function count() {
    counting = true;
    error = "";
    try {
      if (source.countUsage) v = await source.countUsage();
    } catch (e) {
      error = errorText(e);
    } finally {
      counting = false;
    }
  }
  // The beta's plans in words (R3V-Cloud docs/beta.md); another as it is.
  const planName = (p: string) => ({ founding: t("Founding member"), early: t("Early access") } as Record<string, string>)[p] ?? p;
  let top = $derived(Math.max(1, ...(v?.projects ?? []).map((p) => p.bytes)));
</script>

<section>
  {#if v}
    {#if v.hosted}
      <div class="total">
        <strong>{formatBytes(v.used)}</strong>
        {#if v.quota}<span class="muted">{t("of {quota}", { quota: formatBytes(v.quota) })}</span>{/if}
        {#if v.plan}<span class="plan">{planName(v.plan)}</span>{/if}
      </div>
      {#if v.quota}
        <div class="bar" role="meter" aria-valuemin={0} aria-valuemax={v.quota} aria-valuenow={v.used} aria-label={t("Used")}>
          <span style:width="{Math.min(100, (v.used / v.quota) * 100)}%" class:full={v.used > v.quota * 0.9}></span>
        </div>
      {/if}
      <p class="hint">{t("As R3V Cloud counts it for the team's plan.")}</p>
    {:else}
      <div class="total">
        {#if v.counted}
          <strong>{formatBytes(v.used)}</strong>
          <span class="muted">{t("used by the projects' versions")}</span>
        {:else}
          <span class="muted">{t("Not counted yet.")}</span>
        {/if}
        <span class="spacer"></span>
        {#if source.countUsage}<button onclick={count} disabled={counting}>{counting ? t("Counting…") : t("Count now")}</button>{/if}
      </div>
      {#if v.counted}
        <p class="hint">{t("Counted {when}: {versions} versions; the storage holds {stored} in all (files no version uses yet, until a cleanup, too).",
          { when: ago(v.counted), versions: v.versions, stored: formatBytes(v.stored) })}</p>
      {/if}
      <p class="hint">{t("What R3V keeps, as stored (compressed). Your storage company's bill can be more: other things in the bucket, versions it keeps of its own, requests.")}</p>
    {/if}

    {#if v.projects.length}
      <h4>{t("By project")}</h4>
      <ul>
        {#each v.projects as p (p.id)}
          <li>
            <span class="name">{p.name}{#if p.removed} <span class="faint">{t("(removed)")}</span>{/if}</span>
            <span class="pbar"><span style:width="{(p.bytes / top) * 100}%"></span></span>
            <span class="size">{formatBytes(p.bytes)}</span>
          </li>
        {/each}
      </ul>
      {#if !v.hosted}<p class="hint">{t("A file several projects use counts in each of them, and once in the total.")}</p>{/if}
    {/if}
  {:else if !error}
    <p class="faint">{t("Reading…")}</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .total { display: flex; align-items: baseline; gap: var(--sp-8); margin-bottom: var(--sp-8); }
  .total strong { font-size: var(--fs-xl); font-weight: var(--fw-bold); }
  .plan { font-size: var(--fs-xs); padding: 0 var(--sp-6); border-radius: var(--radius-pill); background: var(--panel-2); color: var(--muted); }
  .spacer { flex: 1; }
  .bar { height: 8px; border-radius: var(--radius-pill); background: var(--bg-sunken); overflow: hidden; margin-bottom: var(--sp-6); }
  .bar span { display: block; height: 100%; background: var(--accent); }
  .bar span.full { background: var(--danger); }
  .hint { margin: 0 0 var(--sp-6); font-size: var(--fs-sm); color: var(--faint); }
  h4 { margin: var(--sp-16) 0 var(--sp-8); font-size: var(--fs-sm); color: var(--muted); }
  ul { list-style: none; margin: 0 0 var(--sp-6); padding: 0; display: flex; flex-direction: column; gap: var(--sp-6); }
  li { display: grid; grid-template-columns: minmax(100px, 220px) 1fr auto; align-items: center; gap: var(--sp-10); font-size: var(--fs-md); }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .pbar { height: 6px; border-radius: var(--radius-pill); background: var(--bg-sunken); overflow: hidden; }
  .pbar span { display: block; height: 100%; background: var(--lane-1); }
  .size { font-variant-numeric: tabular-nums; color: var(--muted); font-size: var(--fs-sm); }
  .error { color: var(--danger); }
</style>
