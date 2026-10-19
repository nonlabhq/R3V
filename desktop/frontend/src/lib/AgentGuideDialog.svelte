<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText } from "./api";
  import { toast } from "./notify.svelte";
  import ConfirmDialog from "$shared/ConfirmDialog.svelte";

  // Asks to add the guide for AI agents (AGENTS.md, with a line in
  // CLAUDE.md that reads it) to the project in root. theirs: the project has
  // an AGENTS.md (or CLAUDE.md) of its own, without R3V's part: it goes at
  // its end. added: a project just added, so the guide goes with its first
  // version; else it is to commit.
  let { root, theirs, added = false, onclose, ondone }: {
    root: string; theirs: boolean; added?: boolean; onclose: () => void; ondone?: () => void;
  } = $props();

  async function add() {
    onclose();
    try {
      const written = await api.AddAgentGuide(root);
      const files = written.join(", ");
      toast(added ? t("Added {files}: they go with the project's first version.", { files })
        : t("Added {files}: commit them, so every agent's workspace has them.", { files }), "ok", 8000);
      ondone?.();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
</script>

<ConfirmDialog title={theirs ? t("Add R3V's part to the project's guide for AI agents?") : t("Add a guide for AI agents?")}
  confirm={t("Add the guide")} cancel={t("Not now")}
  text={theirs
    ? t("This project has its own AGENTS.md (or CLAUDE.md). R3V's part goes at its end, the rest left as it is: that the project is versioned with R3V and how to work in it (commit, settle conflicts, work beside other agents).")
    : t("AGENTS.md (with a line in CLAUDE.md that reads it) tells AI agents, Claude Code, Codex and others, that this project is versioned with R3V and how to work in it: commit, settle conflicts, work beside other agents. It's committed with the project.")}
  onconfirm={add} {onclose} />
