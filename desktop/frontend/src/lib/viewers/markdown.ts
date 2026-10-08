// A small Markdown renderer for notes in a project (README, mix notes…):
// headings, paragraphs, lists (and checklists), quotes, code, tables, rules,
// emphasis and links. Everything in the file is escaped first and only these
// tags are made, so HTML in a file shows as text and never runs; links are
// http(s) and mailto only, opened by the app (data-href), never in the page.

const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

// Inline: code first (its insides left as they are), then links, then emphasis.
function inline(text: string): string {
  const codes: string[] = [];
  let s = esc(text).replace(/`([^`]+)`/g, (_, c: string) => `\u0000${codes.push(`<code>${c}</code>`) - 1}\u0000`);
  s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, label: string, url: string) => {
    const u = url.replace(/&amp;/g, "&");
    return /^(https?:\/\/|mailto:)/i.test(u) ? `<a href="#" data-href="${esc(u)}" title="${esc(u)}">${label}</a>` : m;
  });
  s = s.replace(/(\*\*|__)(?=\S)(.+?\S)\1/g, "<strong>$2</strong>")
    .replace(/(^|[^\w*])\*(?=\S)(.+?\S)\*(?!\*)/g, "$1<em>$2</em>")
    .replace(/(^|[^\w_])_(?=\S)(.+?\S)_(?!\w)/g, "$1<em>$2</em>")
    .replace(/~~(?=\S)(.+?\S)~~/g, "<del>$1</del>");
  return s.replace(/\u0000(\d+)\u0000/g, (_, i: string) => codes[+i]);
}

const cells = (row: string) => row.trim().replace(/^\||\|$/g, "").split("|").map((c) => c.trim());

/** The HTML for a Markdown text (safe to insert: see above). */
export function renderMarkdown(src: string): string {
  const lines = src.replace(/\r\n?/g, "\n").split("\n");
  const out: string[] = [];
  let i = 0;
  const isBlockStart = (l: string) => /^(#{1,6}\s|>|```|\s*([-*+]|\d+[.)])\s|\s*(-{3,}|\*{3,}|_{3,})\s*$)/.test(l);
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) { i++; continue; }
    // fenced code
    const fence = /^```/.exec(line);
    if (fence) {
      const body: string[] = [];
      for (i++; i < lines.length && !/^```/.test(lines[i]); i++) body.push(lines[i]);
      i++;
      out.push(`<pre><code>${esc(body.join("\n"))}</code></pre>`);
      continue;
    }
    const h = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(line);
    if (h) { out.push(`<h${h[1].length}>${inline(h[2])}</h${h[1].length}>`); i++; continue; }
    if (/^\s*(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) { out.push("<hr>"); i++; continue; }
    if (/^>/.test(line)) {
      const body: string[] = [];
      for (; i < lines.length && /^>/.test(lines[i]); i++) body.push(lines[i].replace(/^>\s?/, ""));
      out.push(`<blockquote>${renderMarkdown(body.join("\n"))}</blockquote>`);
      continue;
    }
    // a table: a header row, a |---| row, then rows
    if (line.includes("|") && i + 1 < lines.length && /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/.test(lines[i + 1])) {
      const head = cells(line);
      const rows: string[][] = [];
      for (i += 2; i < lines.length && lines[i].includes("|") && lines[i].trim(); i++) rows.push(cells(lines[i]));
      out.push(`<table><thead><tr>${head.map((c) => `<th>${inline(c)}</th>`).join("")}</tr></thead><tbody>` +
        rows.map((r) => `<tr>${head.map((_, k) => `<td>${inline(r[k] ?? "")}</td>`).join("")}</tr>`).join("") + "</tbody></table>");
      continue;
    }
    const li = /^\s*([-*+]|\d+[.)])\s+(.*)$/.exec(line);
    if (li) {
      const ordered = /\d/.test(li[1]);
      const items: string[] = [];
      for (; i < lines.length; i++) {
        const m = /^\s*([-*+]|\d+[.)])\s+(.*)$/.exec(lines[i]);
        if (m && /\d/.test(m[1]) === ordered) items.push(m[2]);
        else if (m || !lines[i].trim() || isBlockStart(lines[i])) break;
        else items[items.length - 1] += " " + lines[i].trim(); // (a line wrapped)
      }
      const item = (t: string) => {
        const task = /^\[([ xX])\]\s+(.*)$/.exec(t);
        return task ? `<li class="task"><input type="checkbox" disabled${task[1] === " " ? "" : " checked"}> ${inline(task[2])}</li>` : `<li>${inline(t)}</li>`;
      };
      out.push(`<${ordered ? "ol" : "ul"}>${items.map(item).join("")}</${ordered ? "ol" : "ul"}>`);
      continue;
    }
    // a paragraph: lines until a blank or another block
    const para: string[] = [];
    for (; i < lines.length && lines[i].trim() && (para.length === 0 || !isBlockStart(lines[i])); i++) para.push(lines[i].trim());
    out.push(`<p>${inline(para.join(" "))}</p>`);
  }
  return out.join("\n");
}
