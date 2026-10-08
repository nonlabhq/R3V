import { t, tn } from "./i18n.svelte";
import type { TeamSummary, Version } from "./api";

// "Yi took back “x”": versions a teammate took back.
export const takenBackText = (vs: Version[]) => tn(vs.length, "{who} took back “{version}”.", "{who} took back {n} versions.",
  { who: [...new Set(vs.map((v) => v.author))].join(", "), version: vs[0]?.message || vs[0]?.short || "" });

// The team's new versions to tell about: a merge only combines the others.
export const newsOf = (incoming: Version[]) => {
  const own = incoming.filter((v) => v.parents.length < 2);
  return own.length ? own : incoming;
};

// Where a team keeps its files, in a word: "R3V Cloud", "R2", "S3".
export const storageKind = (team: Pick<TeamSummary, "hosted" | "address">) =>
  team.hosted ? "R3V Cloud" : /r2\.cloudflarestorage|\.r2\.dev/i.test(team.address) ? "R2"
    : /amazonaws/i.test(team.address) ? "S3" : t("Own storage");
