<script lang="ts">
  import { t } from "./i18n.svelte";
  import { formatBytes, type RuleSuggestion } from "./api";
  import Tx from "./Tx.svelte";

  // A project of another tool found in a folder the rules don't name yet:
  // its preset is suggested (or caches would go up with the version). The
  // text, then the buttons: the parent lays them out in a row (or asks for
  // one part: the notice slot's capsule has its main button apart).
  let { s, onpreset, part }: { s: RuleSuggestion; onpreset: (preset: string) => void; part?: "text" | "main" | "more" } = $props();

  const presetName = (p: string) => ({ ableton: "Ableton Live", unity: "Unity", unreal: "Unreal", godot: "Godot",
    design: t("design"), code: t("code") } as Record<string, string>)[p] ?? p;
  // What a preset leaves out, for people: "Library, Temp, Obj and 9 more".
  const leftOutText = (pats: string[]) => {
    const names = [...new Set(pats.map((p) => p.replace(/^\/|\/$/g, "")))];
    return names.length > 4 ? t("{names} and {n} more", { names: names.slice(0, 4).join(", "), n: names.length - 4 }) : names.join(", ");
  };
</script>

{#if !part || part === "text"}
<div>
  <Tx text={t(s.folder ? "{tool} project found in {folder}." : "{tool} project found in the project folder.")}
    strong={{ folder: `${s.folder}/`, tool: presetName(s.preset) }} />
  <span class="muted" title={s.leftOut.join("  ")}>{s.leftOutBytes > 0
    ? t("Its rules leave out {what} ({size} here).", { what: leftOutText(s.leftOut), size: formatBytes(s.leftOutBytes) })
    : t("Its rules leave out {what}.", { what: leftOutText(s.leftOut) })}</span>
</div>
{/if}
{#snippet main()}<button class="primary" onclick={() => onpreset(s.preset)}>{t("Use {tool} rules", { tool: presetName(s.preset) })}</button>{/snippet}
{#snippet more()}<button class="ghost" onclick={() => onpreset("none")} title={t("R3V won't ask about this folder again")}>{t("Not a project")}</button>{/snippet}
{#if !part}
<div class="row">
  {@render main()}
  {@render more()}
</div>
{:else if part === "main"}{@render main()}{:else if part === "more"}{@render more()}{/if}
