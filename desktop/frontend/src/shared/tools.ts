// Agent tools by the names their makers give them (not translated).
const TOOLS: Record<string, string> = { "claude-code": "Claude Code", codex: "Codex", cursor: "Cursor",
  "gemini-cli": "Gemini CLI", copilot: "GitHub Copilot", aider: "Aider" };

/** An agent tool's name as people know it ("claude-code": "Claude Code"); "" when not known. */
export function toolName(tool: string | undefined | null): string {
  if (!tool) return "";
  return TOOLS[tool] ?? tool.split(/[-_]/).filter(Boolean).map((w) => w[0].toUpperCase() + w.slice(1)).join(" ");
}
