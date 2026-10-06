<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { untrack, type Snippet } from "svelte";
  import { api, ago, errorText, formatBytes, type State, type Result, type Preview, type Conflict, type TeamSummary,
    type Progress, type Version, type RuleSuggestion as Suggestion, type SampleSpot } from "./api";
  import { toast } from "./notify.svelte";
  import { watchProject } from "./projectWatch.svelte";
  import { queue } from "./preupload.svelte";
  import { cachedState, rememberState } from "./stateCache";
  import ChangesPanel from "./ChangesPanel.svelte";
  import CommitBox from "./CommitBox.svelte";
  import EditsSummary from "./EditsSummary.svelte";
  import ProjectHeader from "./ProjectHeader.svelte";
  import ProjectBanners from "./ProjectBanners.svelte";
  import RuleSuggestion from "./RuleSuggestion.svelte";
  import { takenBackText } from "./teamText";
  import UndoDialog from "./UndoDialog.svelte";
  import CombineDialog from "./CombineDialog.svelte";
  import ProgressBar from "./ProgressBar.svelte";
  import History from "./History.svelte";
  import HistoryGraph from "./HistoryGraph.svelte";
  import BranchMenu from "./BranchMenu.svelte";
  import Splitter from "./Splitter.svelte";
  import { splitPx } from "./splits.svelte";
  import VersionDetail from "./VersionDetail.svelte";
  import Modal from "./Modal.svelte";
  import PreviewDialog from "./PreviewDialog.svelte";
  import ProjectCheck from "./ProjectCheck.svelte";
  import ConflictDialog from "./ConflictDialog.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import LiveBlockedDialog from "./LiveBlockedDialog.svelte";
  import CommitCheckDialog from "./CommitCheckDialog.svelte";
  import PromptDialog from "./PromptDialog.svelte";
  import LeaveChangesDialog from "./LeaveChangesDialog.svelte";
  import FirstVersionDialog from "./FirstVersionDialog.svelte";
  import ShareVersionsDialog from "./ShareVersionsDialog.svelte";

  // firstShare: the project was just added (to a team, or this computer).
  // With no versions yet, it asks whether to commit (and share) a first one
  // now or after a look through the files; a project with versions shares
  // them right away.
  let { root, refreshKey, teams, firstShare = false, downloaded = false, onchanged, onfirstshared, ondownloadseen, onsettings, settings }: {
    root: string; refreshKey: number; teams: TeamSummary[]; firstShare?: boolean;
    downloaded?: boolean; // just downloaded: show the project's check
    ondownloadseen?: () => void;
    onchanged: () => void; onfirstshared?: () => void;
    onsettings: () => void; // the project's settings (name, rules, …)
    settings?: Snippet; // the Settings tab (without it, onsettings opens them)
  } = $props();

  let st = $state<State | null>(cachedState(untrack(() => root)));
  let loadError = $state("");
  // The tab is remembered per project. Overview: the history graph, then the
  // picked version (or your changes) and its files; Changes and History are
  // the earlier layout, kept for now.
  type Tab = "overview" | "files" | "settings" | "changes" | "history";
  const tabs: Tab[] = ["overview", "files", "settings", "changes", "history"];
  const tabKey = `r3v.tab:${untrack(() => root)}`;
  let tab = $state<Tab>((() => {
    try {
      const t = localStorage.getItem(tabKey);
      return tabs.includes(t as Tab) ? (t as Tab) : "overview";
    } catch {
      return "overview";
    }
  })());
  $effect(() => {
    const t = tab;
    try { localStorage.setItem(tabKey, t); } catch { /* not remembered */ }
  });
  let message = $state("");
  let busy = $state("");
  let progress = $state<Progress | null>(null);
  let refreshing = $state(false);

  // dialogs
  let preview = $state<{ title: string; label: string; data: Preview; run: Action; blocked: string } | null>(null);
  let conflicts = $state<{ items: Conflict[]; run: Action; force: boolean } | null>(null);
  let liveBlocked = $state<{ run: Action; resolutions: Record<string, string>; set: string } | null>(null);
  let newBranch = $state<string | null>(null);
  // Go to version: asked first when there are uncommitted changes.
  let leaving = $state<{ target: Version | null; message: string } | null>(null); // null target: latest
  let keepOpen = $state<string | null>(null); // message for "Make this the latest version"

  type Action = { name: string; call: (res: Record<string, string>, force: boolean) => Promise<Result | null>;
    done: (r: Result) => void; message?: string };

  // Teammates committed on this branch while you were working: preview, then
  // combine, put your work on a branch, or discard it.
  let combine = $state<{ data: Preview; message: string } | null>(null);
  let branchThenCommit = $state(""); // commit this after creating the branch
  let discardFile = $state(""); // one file's changes, after confirming
  let discardAllOpen = $state(false);
  let discardSome = $state<string[] | null>(null); // the ticked changes, after confirming
  let restoreFile = $state<{ path: string; version: string; label: string; source: string } | null>(null);

  // Two steps: the project folder (fast), then the team's side (network),
  // so the page never waits for the team.
  let teamLoading = false;
  let loadedAt = 0;
  async function load() {
    const r = root;
    loadedAt = Date.now();
    let local: State;
    try {
      local = (await api.State(r))!;
      // Until the team answers (TeamState), what it said last stays when
      // nothing changed here (same version, same branch): otherwise banners
      // that depend on it (not shared yet, new versions) would blink at each
      // reload, e.g. on every focus while the window is resized.
      const prev = st;
      const same = !!prev && prev.root === r && prev.teamChecked && prev.head === local.head && prev.branch === local.branch;
      if (same) local = { ...local, online: prev.online, offline: prev.offline, branches: prev.branches, incoming: prev.incoming,
        takenBack: prev.takenBack, history: prev.history, olderVersion: prev.olderVersion, unshared: prev.unshared,
        teamChecked: true } as State;
      st = local;
      rememberState(local);
      loadError = "";
    } catch (e) {
      loadError = errorText(e);
      return;
    }
    if (!local.remoteUrl || teamLoading) return;
    teamLoading = true;
    try {
      const t = await api.TeamState(r);
      if (t && st?.root === r) {
        st = { ...st, ...t, teamChecked: true } as State;
        rememberState(st);
      }
    } catch {
      // shown as "not reachable" by the next round
    } finally {
      teamLoading = false;
    }
  }

  $effect(() => {
    root; refreshKey;
    load();
  });

  // Just added: ask about the first version once the project is read.
  let firstAsk = $state(false);
  let shareAsk = $state(false);
  let firstShareStarted = false;
  $effect(() => {
    if (!st || firstShareStarted || !untrack(() => firstShare)) return;
    firstShareStarted = true;
    onfirstshared?.();
    if (!st.head) askFirstVersion();
    else if (st.remoteUrl) shareAsk = true; // versions already: share them now or later
  });
  // Samples the sets use that are missing (and R3V has a copy of): offered
  // back, into Samples/Imported. Read again when the project changes.
  let spots = $state<SampleSpot[]>([]);
  $effect(() => {
    root; refreshKey; st?.head; st?.changes.length;
    api.SampleSpots(root).then((s) => (spots = s ?? [])).catch(() => (spots = []));
  });
  let restorable = $derived(spots.filter((s) => s.restorable).length);
  let missingSamples = $derived(spots.filter((s) => s.missing).length);
  let restored = false;
  const restoreAction: Action = {
    name: "restore",
    call: (_res, force) => api.BringSamplesIn(root, true, false, force),
    done: (r) => {
      restored = true;
      const n = Number(r.log[0] ?? 0);
      toast(tn(n, "Restored {n} sample into the project: commit to share it", "Restored {n} samples into the project: commit to share them") + reopen(), "ok", 9000);
    },
  };
  async function restoreSamples(thenCommit = false) {
    restored = false;
    await run(restoreAction);
    if (restored && thenCommit) commit(true);
  }

  // Just downloaded, or asked for: the project's check.
  let checkOpen = $state<"" | "downloaded" | "check">("");
  $effect(() => {
    if (st && untrack(() => downloaded)) {
      checkOpen = "downloaded";
      ondownloadseen?.();
    }
  });

  function askFirstVersion() {
    if (!message.trim()) message = t("First version");
    firstAsk = true;
  }

  // Read again every minute (the team's side), when a set is saved or files
  // change; how a long step is going.
  watchProject(() => root, () => !!busy, load, (p) => (progress = p));

  async function refresh() {
    if (refreshing) return;
    refreshing = true;
    await load();
    refreshing = false;
  }

  let incomingIds = $derived(new Set(st?.incoming.map((v) => v.id) ?? []));
  // Overview: what is picked in the graph ("pending": your changes); by
  // default your changes when there are some, else the version you're on.
  let graphPick = $state("");
  let shown = $derived.by(() => {
    if (!st) return "";
    if (graphPick === "pending" ? st.changes.length > 0 : st.history.some((v) => v.id === graphPick)) return graphPick;
    return st.changes.length ? "pending" : st.head || st.history[0]?.id || "";
  });
  let shownVersion = $derived(st?.history.find((v) => v.id === shown));
  let overviewWidth = $state(0);
  let graphWidth = $derived(splitPx("graph", 0.36, overviewWidth, 280, 520));
  // The graph's card fills the tab (inset); the details float over its right
  // part, from graphWidth to the card's edge less GAP.
  const INSET = 16, GAP = 20;

  // Runs an action; handles conflicts (ask, retry with decisions) and a
  // running Live (ask, retry with force).
  async function run(a: Action, resolutions: Record<string, string> = {}, force = false) {
    busy = a.name;
    try {
      const r = await a.call(resolutions, force);
      if (!r) return;
      if (r.action === "behind") {
        openCombine(a.message ?? message); // nothing changed yet: let the user decide
      } else if (r.liveRunning) {
        liveBlocked = { run: a, resolutions, set: r.openSet };
      } else if (r.conflicts.length) {
        conflicts = { items: r.conflicts, run: a, force }; // keep a "Live is closed" confirmation
      } else {
        a.done(r);
        if (r.takenBack?.length) toast(takenBackText(r.takenBack) + " " + t("It's taken out of your files too.") + reopen(), "info", 10000);
        if (r.relinked.length) toast(tn(r.relinked.length, "Relinked {n} sample path for this computer", "Relinked {n} sample paths for this computer"), "info");
      }
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      busy = "";
      progress = null;
      await load();
      onchanged();
    }
  }

  const saveDone = (r: Result) => {
      const text: Record<string, string> = {
        "published": t("Version committed and shared with the team"),
        "saved-locally": t("Version committed on this computer (not shared with a team)"),
        "fast-forward": t("Updated to the team's latest version: nothing of yours was left to commit"),
        "nothing": t("Nothing changed since your last version"),
        "taken-back": t("Nothing changed since your last version"),
      };
      toast(text[r.action] ?? t("Version committed"), r.action === "nothing" ? "info" : "ok");
      if (r.log.length && r.action === "published") toast(t("The team's versions were taken in first; yours comes after them") + reopen(), "info", 9000);
      if (r.action !== "nothing") message = "";
  };

  // Changes left out of the next commit (unticked in the Changes list),
  // per project; a commit starts over with all ticked.
  let excluded = $state<Record<string, boolean>>({});
  // Cleared for another project only: an effect can run again for other
  // reasons (it did on every key typed in the commit box, ticking all the
  // changes back just before a commit).
  let excludedFor = untrack(() => root);
  $effect.pre(() => {
    if (root !== excludedFor) {
      excludedFor = root;
      excluded = {};
    }
  });
  let leftOut = $derived(st?.changes.filter((c) => excluded[c.path]).length ?? 0);
  // The changes to commit, or none for all of them.
  // (a move commits both its places)
  const picked = () => (leftOut && st ? st.changes.filter((c) => !excluded[c.path]).flatMap((c) => (c.from ? [c.path, c.from] : [c.path])) : []);

  const saveAction = (msg: string, combineWithTeam = false): Action => {
    const paths = picked();
    return {
      name: "save",
      message: msg,
      call: (res, force) => api.Save(root, msg, combineWithTeam, res, force, paths),
      done: (r) => { excluded = {}; saveDone(r); },
    };
  };

  // Commit from the commit box: when the team is ahead, ask first.
  async function commit(confirmed = false, rulesAsked = false) {
    if (!message.trim() || busy || (st?.olderVersion && !st.remoteUrl)) return;
    if (st && st.changes.length && leftOut === st.changes.length) return; // nothing ticked
    if (!confirmed && !rulesAsked && st?.rules.suggestions.length) {
      rulesAsk = true;
      return;
    }
    if (!confirmed) {
      // Files the rules now leave out, and what the project's checks warn
      // about (e.g. a Unity asset without its .meta): say so first.
      const leaving = st?.changes.filter((c) => c.status === "untracked").map((c) => c.path) ?? [];
      const warnings = (await api.CommitWarnings(root).catch(() => [])) ?? [];
      if (leaving.length || warnings.length || restorable) {
        untrackedConfirm = leaving;
        commitWarnings = warnings;
        return;
      }
    }
    // Teammates' versions are taken in first, by the save itself: yours
    // comes after them. Changes made on an older version are combined.
    if (st?.olderVersion) openCombine(message);
    else run(saveAction(message, !!st?.incoming.length));
  }

  // The tool the project is made with: Live gets its own words.
  let isLive = $derived(st?.tool === "Ableton Live");
  // While a version's files are read (looking for changes, adding them to
  // the history), the changes are dimmed: they're being taken as they are.
  let reading = $derived(progress?.stage === "scanning" || progress?.stage === "storing");
  // What to do after R3V changed the project's files.
  const reopen = () => (st?.tool === "Ableton Live" ? " — " + t("reopen the set in Live")
    : st?.tool ? " — " + t("switch back to {tool} to load the changes", { tool: t(st.tool) }) : "");

  // Projects of tools found in folders the rules don't name yet: their
  // preset is suggested (RuleSuggestion), and asked about before committing.
  let rulesAsk = $state(false);
  async function setPreset(s: Suggestion, preset: string) {
    try {
      await api.SetPreset(root, s.folder, preset);
      await refresh();
      if (rulesAsk && !st?.rules.suggestions.length) {
        rulesAsk = false;
        commit(false, true);
      }
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  // The project's rules (.r3v.yaml): files no longer tracked, the dialog.
  let untrackedConfirm = $state<string[] | null>(null);
  let commitWarnings = $state<string[]>([]);
  async function openRules() {
    try {
      await api.OpenRules(root);
      toast(t("Save the file, then R3V follows the new rules"), "info");
    } catch (e) {
      toast(errorText(e), "error");
    }
  }

  async function openCombine(msg: string) {
    busy = "preview";
    try {
      const data = await api.PreviewUpdate(root);
      if (data) combine = { data, message: msg };
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = "";
    }
  }

  function combineAndShare() {
    const msg = combine!.message.trim();
    combine = null;
    message = msg;
    run(saveAction(msg, true));
  }

  function putOnBranch(msg: string) {
    combine = null;
    message = msg.trim();
    branchThenCommit = msg.trim();
    newBranch = "";
  }

  function discardEverything() {
    discardAllOpen = false;
    run({
      name: "discard",
      call: (_res, force) => api.DiscardAll(root, force),
      done: () => toast(t("Discarded all your uncommitted changes") + reopen(), "ok"),
    });
  }

  function discardTicked() {
    const paths = discardSome!;
    discardSome = null;
    run({
      name: "discard",
      call: (_res, force) => api.DiscardFiles(root, paths, force),
      done: () => toast(tn(paths.length, "Discarded your changes to {n} file", "Discarded your changes to {n} files") + reopen(), "ok"),
    });
  }

  function restoreOneFile() {
    const r = restoreFile!;
    restoreFile = null;
    run({
      name: "restore",
      call: (_res, force) => api.RestoreFileVersion(root, r.path, r.version, r.source, force),
      done: () => toast(t("Restored {file} from “{version}” — commit it to keep it", { file: r.path.slice(r.path.lastIndexOf("/") + 1), version: r.label }), "ok", 8000),
    });
  }

  function discardOneFile() {
    const path = discardFile;
    discardFile = "";
    run({
      name: "discard",
      call: (_res, force) => api.DiscardFile(root, path, st?.changes.find((c) => c.path === path)?.from ?? "", force),
      done: () => toast(t("Discarded your changes to {file}", { file: path.slice(path.lastIndexOf("/") + 1) }), "ok"),
    });
  }

  const updateAction: Action = {
    name: "update",
    call: (res, force) => api.Update(root, res, force),
    done: (r) => {
      if (r.action === "fast-forward" || r.action === "merged" || r.action === "taken-back") {
        toast((r.keptWork ? t("You're up to date. Your changes are kept, still uncommitted") : t("You're up to date")) + reopen(), "ok", 8000);
        if (r.action === "merged") toast(t("Your versions and the team's were combined. Commit a version to share the result."), "info", 9000);
      } else toast(t("Already up to date"), "info");
    },
  };

  async function openUpdatePreview() {
    busy = "preview";
    try {
      const data = await api.PreviewUpdate(root);
      mergeMessage = null;
      const own = data?.versions.filter((v) => v.parents.length < 2) ?? [];
      const vs = own.length ? own : (data?.versions ?? []);
      const who = [...new Set(vs.map((v) => v.author))].join(", ");
      if (data) preview = {
        title: who ? tn(vs.length, "{who} shared {n} new version", "{who} shared {n} new versions", { who }) : t("Updates from the team"),
        label: who ? t("Bring in {who}'s changes", { who }) : t("Get updates"), data, run: updateAction,
        blocked: "" };
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = "";
    }
  }

  // The merge version's description, edited in the preview (null: no merge).
  let mergeMessage = $state<string | null>(null);

  async function openMergePreview(name: string) {
    busy = "preview";
    try {
      const data = await api.PreviewMerge(root, name);
      if (!data) return;
      mergeMessage = data.message;
      preview = {
        title: t("Merge “{from}” into “{into}”", { from: name, into: st?.branch ?? "" }), label: t("Merge and share"), data,
        blocked: st?.changes.length ? t("You have uncommitted changes. Commit a version first, then merge.") : "",
        run: {
          name: "merge",
          call: (res, force) => api.MergeBranch(root, name, mergeMessage ?? "", res, force),
          done: (r) => toast(r.action === "up-to-date" || r.action === "ahead"
            ? t("Nothing to merge from {from}", { from: name }) : t("Merged {from} into {into} and shared it", { from: name, into: st?.branch ?? "" }) + reopen(), "ok", 8000),
        },
      };
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = "";
    }
  }

  // Merge any version (e.g. one in the middle of another branch) into the
  // current branch.
  async function openVersionMerge(v: Version) {
    const label = v.branches.length ? v.branches[0] : `“${v.message || v.short}”`;
    busy = "preview";
    try {
      const data = await api.PreviewMergeVersion(root, v.id);
      if (!data) return;
      mergeMessage = data.message;
      preview = {
        title: t("Merge {from} into “{into}”", { from: label, into: st?.branch ?? "" }), label: t("Merge and share"), data,
        blocked: st?.changes.length ? t("You have uncommitted changes. Commit a version first, then merge.") : "",
        run: {
          name: "merge",
          call: (res, force) => api.MergeVersion(root, v.id, mergeMessage ?? "", res, force),
          done: (r) => toast(r.action === "up-to-date" || r.action === "ahead"
            ? t("“{into}” already has {from}", { into: st?.branch ?? "", from: label }) : t("Merged {from} into {into} and shared it", { from: label, into: st?.branch ?? "" }) + reopen(), "ok", 8000),
        },
      };
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = "";
    }
  }

  function switchTo(name: string) {
    run({
      name: "switch",
      call: (_res, force) => api.SwitchBranch(root, name, force),
      done: () => toast(t("Now working on “{branch}”", { branch: name }) + reopen(), "ok", 8000),
    });
  }

  // --- versions: go to, back to latest, keep, export ---

  const goAction = (id: string, discard: boolean, label: string): Action => ({
    name: "goto",
    call: (_res, force) => api.GoToVersion(root, id, discard, force),
    done: () => {
      toast((id === "latest" ? t("Back to the latest version") : t("Now on “{version}”", { version: label })) + reopen(), "ok", 8000);
      if (branchAfterGo === id) { branchAfterGo = ""; newBranch = ""; }
    },
  });
  // New branch from a version: go to it first (a branch starts where you
  // are), then name the branch.
  let branchAfterGo = $state("");
  function newBranchFrom(v: Version) {
    if (v.id === st?.head) { newBranch = ""; return; }
    branchAfterGo = v.id;
    goTo(v);
  }

  // Go to a version (null: back to the latest), asking first about
  // uncommitted changes.
  function goTo(v: Version | null) {
    if (st?.changes.length) {
      leaving = { target: v, message: "" };
      return;
    }
    run(goAction(v?.id ?? "latest", false, v?.message ?? ""));
  }

  async function commitThenGo() {
    const l = leaving!;
    leaving = null;
    let committed = false;
    await run({
      name: "save",
      message: l.message,
      call: (res, force) => api.Save(root, l.message, false, res, force, []),
      done: () => (committed = true),
    });
    if (committed) run(goAction(l.target?.id ?? "latest", false, l.target?.message ?? ""));
  }

  function discardThenGo() {
    const l = leaving!;
    leaving = null;
    run(goAction(l.target?.id ?? "latest", true, l.target?.message ?? ""));
  }

  function keepThisVersion() {
    const msg = (keepOpen ?? "").trim();
    keepOpen = null;
    run({
      name: "keep",
      call: (res) => api.KeepThisVersion(root, msg, res),
      done: () => toast(t("This version is now the latest"), "ok"),
    });
  }

  // Undo commit: asked first (UndoDialog), then done like a save: Live
  // closed, conflicts with later versions decided.
  let undoing = $state<Version | null>(null);
  function undoCommit(v: Version, msg: string, takeBack: boolean) {
    undoing = null;
    const version = v.message || v.short;
    if (takeBack) {
      run({
        name: "undo",
        call: () => api.TakeBackVersion(root, v.id, true),
        done: (r) => toast(r.action === "taken-back"
          ? t("“{version}” is gone from the history, yours and the team's. Its changes are back in your uncommitted changes.", { version })
          : t("“{version}” is gone from the history. Its changes are back in your uncommitted changes.", { version }), "ok", 9000),
      });
      return;
    }
    run({
      name: "undo",
      message: msg,
      call: (res, force) => api.UndoCommit(root, v.id, msg, res, force),
      done: (r) => toast((r.action === "saved-locally" ? t("Undone: a new version takes back “{version}”", { version })
        : t("Undone and shared: a new version takes back “{version}”", { version })) + reopen(), "ok", 8000),
    });
  }

  async function exportVersion(v: Version) {
    const parent = await api.ChooseFolder(t("Where should the copy of this version go?"));
    if (!parent) return;
    busy = "export";
    try {
      const dir = await api.ExportVersion(root, v.id, parent);
      toast(t("Saved a copy of “{version}” as {folder}", { version: v.message || v.short, folder: dir }), "ok", 9000);
      api.ShowFolder(dir);
    } catch (e) {
      toast(errorText(e), "error", 9000);
    } finally {
      busy = "";
      progress = null;
    }
  }

  async function createBranch() {
    const name = (newBranch ?? "").trim();
    if (!name) return;
    busy = "branch";
    try {
      await api.CreateBranch(root, name);
      newBranch = null;
      const msg = branchThenCommit;
      branchThenCommit = "";
      if (msg) {
        busy = "";
        await run({ ...saveAction(msg), done: (r) => { saveDone(r);
          toast(t("Your work is on the new branch “{branch}”; “{base}” is unchanged. Merge it when you're ready.", { branch: name, base: st?.branch ?? "" }), "info", 9000); } });
        return;
      }
      toast(t("Created “{branch}”. Versions you save now go there.", { branch: name }), "ok");
      await load();
      onchanged();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      busy = "";
    }
  }

  // Shares the versions of a project that joined a team (the files aren't
  // committed: what isn't yet stays in Changes).
  function shareVersions() {
    shareAsk = false;
    run({
      name: "first-share",
      call: () => api.ShareVersions(root),
      done: () => {
        toast(t("“{name}” is shared with {team}", { name: st?.name ?? folderName, team: st?.teamName || t("the team") }), "ok");
      },
    });
  }

  let folderName = $derived(root.split(/[\\/]/).pop()?.replace(/ Project$/, "") ?? root);
</script>

<svelte:window onfocus={() => { if (!busy && Date.now() - loadedAt > 3000) load(); }}
  onkeydown={(e) => { if (e.key === "F5" || ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === "r")) refresh(); }} />

{#if loadError && !st && !busy}
  <div class="pad"><p class="error">{loadError}</p></div>
{:else if !st}
  <div class="preparing">
    <h1>{folderName}</h1>
    {#if busy === "first-share"}
      <p class="muted">{isLive ? t("Sharing with the team: R3V commits a first version and uploads it, samples included. Large projects can take a few minutes — you can keep using R3V meanwhile.")
        : t("Sharing with the team: R3V commits a first version and uploads it. Large projects can take a few minutes — you can keep using R3V meanwhile.")}</p>
    {:else}
      <p class="muted">{t("Reading the project…")}</p>
    {/if}
    {#if progress}<ProgressBar p={progress} />{/if}
  </div>
{:else}
  <div class="view">
    <ProjectHeader {st} {refreshing} oncheck={() => (checkOpen = "check")} onrefresh={refresh} />

    <ProjectBanners {st} {busy} {progress} {restorable} {missingSamples} onshare={shareVersions}
      onrecover={() => run({ name: "goto", message: "",
        call: (_res, force) => api.RecoverSwitch(root, force),
        done: () => toast(t("Files put back as they were"), "ok") })}
      onpreset={setPreset} onbranchhere={() => putOnBranch(message || "")} onlatest={() => goTo(null)}
      oncombine={() => openCombine(message)} onnewbranch={() => (newBranch = "")}
      onkeep={() => (keepOpen = t("Back to “{version}”", { version: st!.olderVersion!.message || st!.olderVersion!.short }))}
      onupdate={() => run(updateAction)} onpreview={openUpdatePreview} onrestore={() => restoreSamples()} onopenrules={openRules}
      onqueue={() => (queue.open = true)} {loadError} />


    <nav>
      <button class:on={tab === "overview"} onclick={() => (tab = "overview")}>
        {t("Overview")} {#if st.changes.length}<span class="count">{st.changes.length}</span>{/if}
      </button>
      <button class:on={tab === "files"} onclick={() => (tab = "files")}>{t("Files")}</button>
      {#if settings}
        <button class:on={tab === "settings"} onclick={() => (tab = "settings")}
          title={st.rules.error ? `⚠ ${st.rules.error}` : undefined}>{t("Settings")}{#if st.rules.error} <span class="bad">⚠</span>{/if}</button>
      {:else}
        <button onclick={onsettings}>{t("Settings")}</button>
      {/if}
      <span class="sep" aria-hidden="true"></span>
      <button class:on={tab === "changes"} onclick={() => (tab = "changes")}>
        {t("Changes")} {#if st.changes.length}<span class="count">{st.changes.length}</span>{/if}
      </button>
      <button class:on={tab === "history"} onclick={() => (tab = "history")}>{t("History")}</button>
    </nav>

    {#snippet changesPanel(scope: "changes" | "all" | undefined, footer = false)}
      {#snippet summary()}<EditsSummary st={st!} />{/snippet}
      {#snippet commitBox()}<CommitBox st={st!} bind:message {busy} {leftOut} oncommit={() => commit()} />{/snippet}
      <ChangesPanel {root} st={st!} {summary} commitBox={scope === "all" || footer ? undefined : commitBox} {scope} bind:excluded
        onrules={() => load()} ondiscard={(p) => (discardFile = p)}
        ondiscardall={() => (discardAllOpen = true)} ondiscardsome={(paths) => (discardSome = paths)}
        onrestore={(path, version, label, source) => (restoreFile = { path, version, label, source })} />
    {/snippet}

    {#snippet versionActions(v: Version)}
      {#if st!.remoteUrl && !st!.olderVersion && !v.inBranch && !incomingIds.has(v.id)}
        <button onclick={() => openVersionMerge(v)} title={t("Merge this version into the branch you are on")}>{t("Merge")}</button>
      {/if}
      {#if v.id !== st!.head && !incomingIds.has(v.id) && !v.notHere}
        <button onclick={() => goTo(v)} title={t("Put the project in the state of this version")}>{t("Go to")}</button>
      {/if}
      {#if !st!.olderVersion && v.inBranch && !incomingIds.has(v.id) && v.parents.length}
        <button onclick={() => (undoing = v)} title={t("Make a new version that takes back what this version changed")}>{t("Undo commit")}</button>
      {/if}
      {#if !v.notHere}
        <button onclick={() => exportVersion(v)} title={t("Save this version as a separate project folder")}>{t("Export…")}</button>
      {/if}
    {/snippet}

    <main class:flush={tab !== "history" && tab !== "settings"} class:reading inert={reading}>
      {#if tab === "overview"}
        <div class="overview" bind:clientWidth={overviewWidth}>
          <div class="graph-pane">
            <div class="graph-bar">
              <BranchMenu {st} onswitch={switchTo} onmerge={openMergePreview} onnewbranch={() => (newBranch = "")} />
            </div>
            {#snippet cardActions(v: Version)}
              {@const isNew = incomingIds.has(v.id)}
              <button disabled={v.id === st!.head || isNew || v.notHere} onclick={() => goTo(v)}
                title={v.id === st!.head ? t("You are on this version") : isNew ? t("Get updates first") : t("Put the project in the state of this version")}>{t("Go to")}</button>
              <button disabled={!st!.remoteUrl || !!st!.olderVersion || v.inBranch || isNew} onclick={() => openVersionMerge(v)}
                title={v.inBranch ? t("Already in the branch you are on") : t("Merge this version into the branch you are on")}>{t("Merge")}</button>
              <button disabled={!!st!.olderVersion || !v.inBranch || isNew || !v.parents.length} onclick={() => (undoing = v)}
                title={t("Make a new version that takes back what this version changed")}>{t("Undo commit")}</button>
              <button disabled={!st!.remoteUrl || isNew || v.notHere} onclick={() => newBranchFrom(v)}
                title={t("Start a branch from this version")}>{t("New branch")}</button>
            {/snippet}
            <HistoryGraph actions={cardActions} versions={st.history} branches={st.branches.map((b) => ({ name: b.name, latest: b.latest?.id ?? "" }))}
              branch={st.branch} head={st.head} incoming={incomingIds}
              pending={st.changes.length} selected={shown} onselect={(id) => (graphPick = id)}
              reserve={Math.max(0, overviewWidth - graphWidth - INSET)} panelInset={GAP} />
          </div>
          {#if overviewWidth}<Splitter key="graph" def={0.36} width={overviewWidth} minLeft={280} minRight={520} />{/if}
          <div class="detail-pane" style:left="{graphWidth}px" style:top="{INSET + GAP}px" style:right="{INSET + GAP}px" style:bottom="{INSET + GAP}px">
            {#if shown === "pending"}
              <div class="pending-h">
                <strong>{t("Your changes")}</strong>
                <span class="faint">{t("not committed yet · on {branch}", { branch: st.branch })}</span>
              </div>
              <div class="pending-body">{@render changesPanel("changes", true)}</div>
              <div class="panel-foot"><CommitBox st={st} bind:message {busy} {leftOut} oncommit={() => commit()} inline /></div>
            {:else if shownVersion}
              {@const v = shownVersion}
              {#snippet acts()}{@render versionActions(v)}{/snippet}
              <VersionDetail {root} {v} branch={v.inBranch ? st.branch : v.branches.join(", ")} actions={acts} />
            {:else}
              <p class="muted pad">{t("No versions yet. Commit your first version from the Changes tab.")}</p>
            {/if}
          </div>
        </div>
      {:else if tab === "files"}
        {@render changesPanel("all")}
      {:else if tab === "settings" && settings}
        <div class="settings">{@render settings()}</div>
      {:else if tab === "changes"}
        {@render changesPanel(undefined)}
      {:else if tab === "history"}
        <History {root} versions={st.history} head={st.head} incoming={incomingIds} latest={st.latest}
          ongoto={(v) => goTo(v)} onexport={exportVersion}
          onundo={!st.olderVersion ? (v) => (undoing = v) : undefined}
          onmerge={st.remoteUrl && !st.olderVersion ? openVersionMerge : undefined} />
      {/if}
    </main>


  </div>

  {#if shareAsk}
    <ShareVersionsDialog {st} onshare={shareVersions} onclose={() => (shareAsk = false)} />
  {/if}

  {#if checkOpen}
    <Modal title={checkOpen === "downloaded" ? t("“{name}” is downloaded", { name: st.name }) : t("Project check")} onclose={() => (checkOpen = "")}>
      {#if checkOpen === "downloaded"}<p>{t("Can this computer open it? R3V looked:")}</p>{/if}
      <ProjectCheck {root} mode={checkOpen} onrestore={() => { checkOpen = ""; restoreSamples(); }} />
      {#snippet footer()}
        <button class="primary" onclick={() => (checkOpen = "")}>{t("OK")}</button>
      {/snippet}
    </Modal>
  {/if}

  {#if firstAsk}
    <FirstVersionDialog {st} bind:message onclose={() => (firstAsk = false)} oncommit={() => { firstAsk = false; commit(); }} />
  {/if}

  {#if combine}
    <CombineDialog {root} preview={combine.data} branch={st.branch} older={!!st.olderVersion} bind:message={combine.message} busy={!!busy}
      onclose={() => (combine = null)} oncombine={combineAndShare} onbranch={() => putOnBranch(combine!.message)} />
  {/if}

  {#if discardAllOpen}
    <ConfirmDialog title={t("Discard all your changes?")} danger confirm={t("Discard all")}
      text={tn(st.changes.length, "Your {n} uncommitted change will be lost: the project folder goes back to the version you're on. This can't be undone.",
        "All {n} uncommitted changes will be lost: the project folder goes back to the version you're on. This can't be undone.")}
      onconfirm={discardEverything} onclose={() => (discardAllOpen = false)} />
  {/if}

  {#if discardSome}
    {@const n = discardSome.length}
    <ConfirmDialog title={tn(n, "Discard {n} ticked change?", "Discard {n} ticked changes?")} danger confirm={t("Discard")}
      text={tn(n, "The file goes back to how it is in the version you're on. The unticked changes stay. This can't be undone.",
        "These {n} files go back to how they are in the version you're on. The unticked changes stay. This can't be undone.")}
      onconfirm={discardTicked} onclose={() => (discardSome = null)} />
  {/if}

  {#if restoreFile}
    {@const r = restoreFile}
    <ConfirmDialog title={t("Restore {file} from “{version}”?", { file: r.path.slice(r.path.lastIndexOf("/") + 1), version: r.label })}
      text={t("The file goes back to how it was in that version. The rest of the project stays as it is; commit when you're happy with it.")}
      warn={st.changes.some((c) => c.path === r.path) ? t("This file has uncommitted changes — they'll be replaced.") : ""}
      confirm={t("Restore")} onconfirm={restoreOneFile} onclose={() => (restoreFile = null)} />
  {/if}

  {#if discardFile}
    {@const movedFrom = st.changes.find((c) => c.path === discardFile)?.from}
    <ConfirmDialog title={t("Discard your changes to {file}?", { file: discardFile.slice(discardFile.lastIndexOf("/") + 1) })}
      text={movedFrom ? t("The file goes back to {path}, as it is in the version you're on. This can't be undone.", { path: movedFrom })
        : t("The file goes back as it is in the version you're on. This can't be undone.")}
      danger confirm={t("Discard changes")} onconfirm={discardOneFile} onclose={() => (discardFile = "")} />
  {/if}

  {#if rulesAsk && st?.rules.suggestions.length}
    <Modal title={t("Before you commit")} onclose={() => (rulesAsk = false)}>
      <p>{st.rules.suggestions.length === 1 ? t("R3V found a project of another tool in this one.") : t("R3V found projects of other tools in this one.")}
        {t("A tool's rules leave out what it makes again by itself (caches, backups), so that doesn't go up with the version.")}</p>
      {#each st.rules.suggestions as s (s.folder + s.preset)}
        <div class="suggestion"><RuleSuggestion {s} onpreset={(p) => setPreset(s, p)} /></div>
      {/each}
      {#snippet footer()}
        <button onclick={() => (rulesAsk = false)}>{t("Cancel")}</button>
        <button onclick={() => { rulesAsk = false; commit(false, true); }}>{t("Commit without these rules")}</button>
      {/snippet}
    </Modal>
  {/if}

  {#if untrackedConfirm}
    <CommitCheckDialog leaving={untrackedConfirm} warnings={commitWarnings} {restorable} {missingSamples}
      onclose={() => (untrackedConfirm = null)}
      oncommit={() => { untrackedConfirm = null; commit(true); }}
      onrestore={() => { untrackedConfirm = null; restoreSamples(true); }} />
  {/if}

  {#if preview}
    {@const p = preview}
    <PreviewDialog {root} keepsWork={p.run === updateAction && !!st.changes.length} title={p.title} preview={p.data} actionLabel={p.label} blocked={p.blocked} bind:message={mergeMessage}
      onclose={() => (preview = null)}
      onconfirm={() => { const action = p.run; preview = null; run(action); }} />
  {/if}

  {#if conflicts}
    {@const c = conflicts}
    <ConflictDialog conflicts={c.items} onclose={() => (conflicts = null)}
      onresolve={(res) => { const { run: action, force } = c; conflicts = null; run(action, res, force); }} />
  {/if}

  {#if liveBlocked}
    {@const b = liveBlocked}
    <LiveBlockedDialog tool={st.tool} set={b.set} onclose={() => (liveBlocked = null)}
      oncontinue={() => { const { run: action, resolutions } = b; liveBlocked = null; run(action, resolutions); }} />
  {/if}

  {#if leaving}
    {@const l = leaving}
    <LeaveChangesDialog changes={st.changes.length} latest={!l.target} older={!!st.olderVersion} team={!!st.remoteUrl}
      bind:message={l.message} oncommit={commitThenGo} ondiscard={discardThenGo} onclose={() => (leaving = null)} />
  {/if}

  {#if undoing}
    {@const v = undoing}
    <UndoDialog {root} version={v} onclose={() => (undoing = null)} onundo={(msg, takeBack) => undoCommit(v, msg, takeBack)} />
  {/if}

  {#if keepOpen !== null}
    <PromptDialog title={t("Make this the latest version")} label={t("Describe it")} confirm={t("Make it the latest")}
      text={t("The older version you are on (with any changes you made) becomes a new version on top of the latest one. Nothing in the history is lost.")}
      bind:value={() => keepOpen ?? "", (v) => (keepOpen = v)} busy={!!busy}
      onconfirm={keepThisVersion} onclose={() => (keepOpen = null)} />
  {/if}

  {#if newBranch !== null}
    <PromptDialog title={t("New branch")} label={t("Branch name")} placeholder="yi-chorus-idea" confirm={t("Create")}
      text={t("A branch is your own line of versions (e.g. to try an idea). The team keeps working on “{branch}”; merge back when you're happy.", { branch: st.branch })}
      bind:value={() => newBranch ?? "", (v) => (newBranch = v)} busy={busy === "branch"}
      onconfirm={createBranch} onclose={() => (newBranch = null)} />
  {/if}
{/if}

<style>
  .view { display: flex; flex-direction: column; height: 100%; }
  .pad { padding: var(--sp-24); }
  /* Same place as the loaded header's title, so nothing jumps. */
  .preparing { padding: var(--sp-18) var(--sp-24); max-width: 600px; }
  .error { color: var(--danger); }
  h1 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  h1 { margin: 0 0 var(--sp-6); font-size: var(--fs-2xl); font-weight: var(--fw-semibold); }

  .suggestion { display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-10) 0; border-top: var(--border-width) solid var(--line); }
  .suggestion > :global(div:first-child) { flex: 1; }

  nav { display: flex; gap: var(--sp-4); padding: var(--sp-10) var(--sp-24) 0; border-bottom: var(--border-width) solid var(--line); }
  nav button { border: none; background: transparent; border-radius: var(--radius) var(--radius) 0 0; padding: var(--sp-8) var(--sp-14); color: var(--muted); border-bottom: 2px solid transparent; }
  nav button.on { color: var(--text); border-bottom-color: var(--accent); }
  .count { margin-left: var(--sp-4); font-size: var(--fs-xs); padding: 0 var(--sp-6); border-radius: var(--radius-lg); background: var(--hover); }

  main { flex: 1; overflow: auto; padding: var(--sp-16) var(--sp-24) var(--sp-32); }
  main.reading { opacity: .45; transition: opacity .2s; }
  main.flush { padding: 0; overflow: hidden; min-height: 0; }
  nav .sep { width: var(--border-width); align-self: stretch; margin: var(--sp-6) var(--sp-8); background: var(--line); }
  /* Overview: the graph, then the picked version (or your changes) and its files. */
  .overview { position: relative; height: 100%; min-height: 0; }
  /* The graph: one big card (dots 18px apart) filling the tab. */
  .graph-pane { position: absolute; inset: 16px; display: flex; flex-direction: column; min-height: 0; overflow: hidden;
    border-radius: var(--radius-card); background-color: var(--panel); box-shadow: var(--shadow-card);
    background-image: radial-gradient(circle, var(--dot-grid) 1px, transparent 1.4px); background-size: 18px 18px; }
  .graph-pane > :global(:last-child) { flex: 1; min-height: 0; }
  .graph-bar { position: absolute; top: var(--sp-14); left: var(--sp-16); z-index: 2; }
  nav .bad { color: var(--warn); }
  /* The details float over the card: one see-through colour for all of it,
     parts set apart by lines, no frame. */
  .detail-pane { position: absolute; z-index: 2; display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden;
    border-radius: var(--radius-xl); background: var(--surface-float); backdrop-filter: var(--float-filter); box-shadow: var(--shadow-float); }
  .detail-pane :global(.side), .detail-pane :global(.commit), .detail-pane :global(.head), .detail-pane :global(.detail-h) { background: transparent; }
  .detail-pane :global(.head) { position: static; }
  .detail-pane :global(.side), .detail-pane :global(aside) { border-color: var(--line-strong); }
  .pending-h { display: flex; align-items: baseline; gap: var(--sp-10); padding: var(--sp-14) var(--sp-16);
    border-bottom: var(--border-width) solid var(--line-strong); }
  .pending-h strong { color: var(--accent); font-style: italic; font-size: var(--fs-lg); }
  .panel-foot { flex: none; padding: var(--sp-12) var(--sp-14); border-top: var(--border-width) solid var(--line-strong); }
  .pending-h .faint { font-size: var(--fs-sm); }
  .pending-body { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  .pending-body > :global(*) { flex: 1; min-height: 0; }
  .settings { max-width: 680px; }
  .pad { padding: var(--sp-16); }

</style>
