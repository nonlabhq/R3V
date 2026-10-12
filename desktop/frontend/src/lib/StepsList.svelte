<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, ago, errorText, type Session, type Step, type StepChange, type TalkEntry } from "./api";
  import { toast } from "./notify.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import Modal from "./Modal.svelte";

  // A session as it went: each prompt (the latest first), folded to its
  // time and words, its toggle opening what the agent said and the steps it
  // made, in order. The agent's words and what
  // each call did come from the conversation the agent tool keeps (read
  // when shown, never copied). Compare two steps; go back to one when
  // onback is given (the work you're on, not a version's steps or another
  // copy's).
  let { root, session, busy = false, openLatest = false, onback }: {
    root: string; // the copy whose steps they are
    session: Session;
    busy?: boolean;
    openLatest?: boolean; // the latest prompt open at first (the work going on)
    onback?: (session: string, n: number) => void;
  } = $props();

  let asking = $state<number | null>(null);
  let from = $state<number | null>(null);
  let diff = $state<{ a: number; b: number; changes: StepChange[] } | null>(null);
  let open = $state<Record<string, boolean>>({}); // words shown whole
  let unfolded = $state<Record<number, boolean>>({}); // prompts open, by number
  const isOpen = (n: number, i: number) => unfolded[n] ?? (openLatest && i === session.turns.length - 1);
  const when = (s: string) => new Date(s).toLocaleString([], { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" });

  // The conversation, read once per session.
  let talk = $state<TalkEntry[]>([]);
  $effect(() => {
    const id = session.id, at = root;
    talk = [];
    api.StepsTalk(at, id).then((x) => { if (id === session.id) talk = x ?? []; }).catch(() => {});
  });

  const ms = (s: string) => Date.parse(s) || 0;
  type Item = { kind: "reply"; time: number; text: string; key: string } | { kind: "step"; time: number; step: Step; detail: string; key: string };

  // A turn's items in order: the agent's replies between its prompt and the
  // next, and its steps, each with what its call did.
  function itemsOf(i: number): Item[] {
    const turns = session.turns;
    const start = ms(turns[i].time) - 5000;
    const end = i + 1 < turns.length ? ms(turns[i + 1].time) - 5000 : Infinity;
    const inTurn = talk.filter((e) => { const x = ms(e.time); return x >= start && x < end; });
    const items: Item[] = inTurn.filter((e) => e.kind === "reply")
      .map((e, n) => ({ kind: "reply", time: ms(e.time), text: e.text ?? "", key: `r${i}-${n}` }));
    let prev = start;
    for (const st of turns[i].steps) {
      const at = ms(st.time) + 1500;
      const calls = inTurn.filter((e) => e.kind === "tool" && ms(e.time) > prev && ms(e.time) <= at);
      const call = [...calls].reverse().find((e) => e.tool === st.tool) ?? calls.at(-1);
      items.push({ kind: "step", time: ms(st.time), step: st, detail: st.detail || call?.detail || "", key: `s${st.n}` });
      prev = at;
    }
    return items.sort((a, b) => a.time - b.time);
  }

  // A message another session relayed (Claude Code wraps it): who from, and its words.
  function promptOf(p: string): { from: string; text: string } {
    const m = /^\s*<cross-session-message\b([^>]*)>([\s\S]*?)(?:<\/cross-session-message>\s*)?$/.exec(p);
    if (!m) return { from: "", text: p };
    return { from: /from-name="([^"]*)"/.exec(m[1])?.[1] ?? "", text: m[2].trim() };
  }

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
      diff = { a, b, changes: (await api.StepDiff(root, session.id, a, b)) ?? [] };
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  const statusText = (st: string) => ({ added: t("added"), modified: t("changed"), deleted: t("deleted") } as Record<string, string>)[st] ?? st;
  const toggle = (key: string) => (open[key] = !open[key]);
</script>

<div class="turns">
  {#each [...session.turns.keys()].reverse() as i (session.turns[i].n)}
    {@const turn = session.turns[i]}
    {@const shown = isOpen(turn.n, i)}
    {@const p = promptOf(turn.prompt || "")}
    <div class="turn" class:shown>
      <button class="turn-h" aria-expanded={shown} onclick={() => (unfolded[turn.n] = !shown)}>
        <span class="chev" aria-hidden="true">{shown ? "▾" : "▸"}</span>
        <span class="when">{when(turn.time)}</span>
        <span class="gist" class:note={!turn.prompt}>{#if p.from}<span class="from">{`${p.from} · `}</span>{/if}{turn.prompt ? p.text : turn.note || t("(no prompt)")}</span>
        <span class="count">{tn(turn.steps.length, "{n} step", "{n} steps")}</span>
      </button>
      {#if shown}
      <div class="body">
      {#if turn.prompt}
        <p class="whole-prompt">{p.text}</p>
      {/if}
      {#each itemsOf(i) as it (it.key)}
        {#if it.kind === "reply"}
          <button class="words reply" class:whole={open[it.key]} onclick={() => toggle(it.key)} title={t("Show it all")}>
            <span class="text">{it.text}</span>
          </button>
        {:else}
          {@const st = it.step}
          <div class="step" class:viewing={!onback}>
            <span class="n">{st.n}</span>
            <span class="tool" title={it.detail}>{toolLabel(st.tool)}{#if it.detail && !st.files.includes(it.detail)}<span class="detail">{` · ${it.detail}`}</span>{/if}</span>
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
              {#if onback}<button class="small" disabled={busy} onclick={() => (asking = st.n)}>{t("Go back")}</button>{/if}
            </span>
          </div>
        {/if}
      {/each}
      </div>
      {/if}
    </div>
  {/each}
</div>

{#if asking !== null}
  <ConfirmDialog title={t("Go back to step {n}?", { n: asking })}
    text={t("The project's files go back to how they were at step {n}. The files as they are now are kept as a step first, so you can come back to them.", { n: asking })}
    confirm={t("Go back")} onconfirm={() => { const n = asking!; asking = null; onback?.(session.id, n); }} onclose={() => (asking = null)} />
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
  .turn { margin: var(--sp-2) 0; }
  .turn-h { display: grid; grid-template-columns: 14px auto minmax(0, 1fr) auto; align-items: baseline; gap: var(--sp-8); width: 100%;
    padding: var(--sp-6) var(--sp-4); border: none; border-radius: var(--radius); background: none; color: inherit; font: inherit;
    font-size: var(--fs-sm); text-align: left; cursor: pointer; }
  .turn-h:hover { background: var(--hover); }
  .chev { color: var(--faint); font-size: var(--fs-xs); }
  .when { color: var(--faint); font-variant-numeric: tabular-nums; white-space: nowrap; }
  .gist { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .gist.note { font-style: italic; color: var(--muted); }
  .gist .from { color: var(--faint); }
  .count { color: var(--faint); white-space: nowrap; }
  .body { padding: var(--sp-4) 0 var(--sp-8) calc(14px + var(--sp-12)); }
  .whole-prompt { margin: 0 0 var(--sp-6); font-size: var(--fs-sm); font-weight: var(--fw-semibold); white-space: pre-wrap; }
  .words { display: block; width: 100%; text-align: left; background: none; border: none; padding: var(--sp-2) 0; color: inherit;
    font: inherit; font-size: var(--fs-sm); cursor: pointer; }
  .words .text { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; line-clamp: 2; overflow: hidden; white-space: pre-wrap; }
  .words.whole .text { display: block; -webkit-line-clamp: unset; line-clamp: unset; }
  .reply { color: var(--muted); padding-left: var(--sp-12); border-left: 2px solid var(--line); margin: var(--sp-4) 0; }
  .step { display: grid; grid-template-columns: 28px minmax(80px, 2fr) minmax(0, 2fr) auto auto; align-items: center; gap: var(--sp-8);
    padding: var(--sp-2) 0; font-size: var(--fs-sm); }
  .n { color: var(--faint); text-align: right; font-variant-numeric: tabular-nums; }
  .tool { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .detail { color: var(--muted); }
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
