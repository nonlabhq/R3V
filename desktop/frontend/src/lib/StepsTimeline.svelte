<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, ago, errorText, type StepsInfo, type StepChange } from "./api";
  import { toast } from "./notify.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import Modal from "./Modal.svelte";

  // An agent's steps (docs/design/steps.md): the project after each of its
  // moves, under the prompt that asked for them; shown above Your changes.
  // Read again when the project changes (version), and now and then: agents
  // work without telling the app.
  let { root, version, busy = false, onback }: {
    root: string;
    version: string;
    busy?: boolean;
    onback: (session: string, n: number) => void; // go back to step n (the view runs it: Live's check, the result)
  } = $props();

  let info = $state<StepsInfo | null>(null);
  let session = $state(""); // "" the latest
  let open = $state(false);
  let asking = $state<number | null>(null); // go back to this step?
  let from = $state<number | null>(null); // compare: the first step picked
  let diff = $state<{ a: number; b: number; changes: StepChange[] } | null>(null);

  async function load() {
    const r = root;
    try {
      const got = await api.Steps(r, session);
      if (r === root) info = got;
    } catch {
      // (no steps to show: the timeline stays as it was)
    }
  }
  $effect(() => {
    root; version; session;
    load();
  });
  $effect(() => {
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  });

  const s = $derived(info?.session ?? null);
  const latestPrompt = $derived.by(() => {
    for (const turn of [...(s?.turns ?? [])].reverse()) if (turn.prompt || turn.note) return turn.prompt || turn.note;
    return "";
  });

  // What made a step, in words: the agent tool's name for it, or R3V's.
  function toolLabel(tool: string): string {
    if (tool === "r3v step") return t("By hand");
    if (tool === "before going back") return t("Before going back");
    const back = /^back to step (\d+)$/.exec(tool);
    if (back) return t("Back to step {n}", { n: back[1] });
    const mcp = /^mcp__([^_]+(?:_[^_]+)*)__(.+)$/.exec(tool);
    if (mcp) return `${mcp[1]} · ${mcp[2]}`;
    return tool;
  }
  const filesText = (files: string[]) =>
    files.length <= 2 ? files.join(", ") : t("{files} and {n} more", { files: files.slice(0, 2).join(", "), n: files.length - 2 });

  async function compare(a: number, b: number) {
    from = null;
    try {
      diff = { a, b, changes: (await api.StepDiff(root, s!.id, a, b)) ?? [] };
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  const statusText = (st: string) => ({ added: t("added"), modified: t("changed"), deleted: t("deleted") } as Record<string, string>)[st] ?? st;
</script>

{#if info?.available && info.on && s}
  <section class="steps">
    <button class="head" onclick={() => (open = !open)} aria-expanded={open}>
      <span class="chev">{open ? "▾" : "▸"}</span>
      <strong>{t("Steps")}</strong>
      <span class="faint">{tn(s.steps, "{n} step", "{n} steps")}{latestPrompt ? ` · “${latestPrompt}”` : ""}</span>
    </button>
    {#if open}
      {#if (info.sessions?.length ?? 0) > 1}
        <label class="session">
          <span class="faint">{t("Session")}</span>
          <select bind:value={session}>
            <option value="">{t("The latest")}</option>
            {#each info.sessions as o (o.id)}
              <option value={o.id}>{o.prompt ? o.prompt.slice(0, 60) : o.id} · {ago(o.last)}</option>
            {/each}
          </select>
        </label>
      {/if}
      <div class="turns">
        {#each [...s.turns].reverse() as turn (turn.n)}
          <div class="turn">
            {#if turn.prompt}<p class="prompt" title={turn.prompt}>“{turn.prompt}”</p>
            {:else if turn.note}<p class="prompt note">{turn.note}</p>{/if}
            {#each [...turn.steps].reverse() as st (st.n)}
              <div class="step">
                <span class="n">{st.n}</span>
                <span class="tool">{toolLabel(st.tool)}</span>
                <span class="files faint" title={st.files.join("\n")}>{filesText(st.files)}</span>
                <span class="time faint">{ago(st.time)}</span>
                <span class="acts">
                  {#if from === null}
                    <button class="ghost small" onclick={() => (from = st.n)}>{t("Compare")}</button>
                  {:else if from === st.n}
                    <button class="ghost small" onclick={() => (from = null)}>{t("Cancel")}</button>
                  {:else}
                    <button class="small" onclick={() => compare(Math.min(from!, st.n), Math.max(from!, st.n))}>{t("Compare with step {n}", { n: from })}</button>
                  {/if}
                  <button class="small" disabled={busy} onclick={() => (asking = st.n)}>{t("Go back")}</button>
                </span>
              </div>
            {/each}
          </div>
        {/each}
      </div>
    {/if}
  </section>
{/if}

{#if asking !== null}
  <ConfirmDialog title={t("Go back to step {n}?", { n: asking })}
    text={t("The project's files go back to how they were at step {n}. The files as they are now are kept as a step first, so you can come back to them.", { n: asking })}
    confirm={t("Go back")} onconfirm={() => { const n = asking!; asking = null; onback(s!.id, n); }} onclose={() => (asking = null)} />
{/if}

{#if diff}
  <Modal title={t("From step {a} to step {b}", { a: diff.a, b: diff.b })} width={520} onclose={() => (diff = null)}>
    {#if diff.changes.length === 0}
      <p class="faint">{t("No files changed.")}</p>
    {:else}
      <ul class="changes">
        {#each diff.changes as c (c.path)}
          <li><span class="st {c.status}">{statusText(c.status)}</span> <span class="path">{c.path}</span></li>
        {/each}
      </ul>
    {/if}
    {#snippet footer()}<button onclick={() => (diff = null)}>{t("Close")}</button>{/snippet}
  </Modal>
{/if}

<style>
  .steps { border-bottom: var(--border-width) solid var(--line); padding: var(--sp-6) 0; }
  .head { display: flex; align-items: baseline; gap: var(--sp-8); width: 100%; background: none; border: none; padding: var(--sp-4) var(--sp-2);
    cursor: pointer; text-align: left; color: inherit; overflow: hidden; white-space: nowrap; }
  .head .faint { overflow: hidden; text-overflow: ellipsis; }
  .chev { width: 12px; color: var(--faint); }
  .session { display: flex; align-items: center; gap: var(--sp-8); margin: var(--sp-6) 0 var(--sp-4) var(--sp-20); }
  .turns { max-height: 320px; overflow-y: auto; padding-left: var(--sp-20); }
  .turn { margin: var(--sp-8) 0; }
  .prompt { margin: 0 0 var(--sp-4); font-size: var(--fs-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .prompt.note { font-style: italic; }
  .step { display: grid; grid-template-columns: 28px minmax(80px, auto) 1fr auto auto; align-items: center; gap: var(--sp-8);
    padding: var(--sp-2) 0; font-size: var(--fs-sm); }
  .n { color: var(--faint); text-align: right; font-variant-numeric: tabular-nums; }
  .tool { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .files { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }
  .time { white-space: nowrap; }
  .acts { display: flex; gap: var(--sp-4); }
  .changes { list-style: none; margin: 0; padding: 0; }
  .changes li { padding: var(--sp-2) 0; }
  .st { display: inline-block; min-width: 64px; color: var(--faint); }
  .st.added { color: var(--ok); }
  .st.deleted { color: var(--danger); }
  .path { word-break: break-all; }
</style>
