import { describe, expect, it } from "vitest";
import { renderMarkdown as md } from "./markdown";

describe("markdown", () => {
  it("never lets HTML or script links through", () => {
    expect(md("<script>alert(1)</script>")).toBe("<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>");
    expect(md('<img src=x onerror="alert(1)">')).not.toContain("<img");
    expect(md("[x](javascript:alert(1))")).not.toContain("<a");
    expect(md("[site](https://example.com)")).toBe('<p><a href="#" data-href="https://example.com" title="https://example.com">site</a></p>');
    expect(md('[x](https://e.com/"onmouseover=alert(1))')).not.toMatch(/data-href="[^"]*"[^ >]/);
    expect(md("```\n<b>x</b>\n```")).toBe("<pre><code>&lt;b&gt;x&lt;/b&gt;</code></pre>");
  });

  it("draws headings, emphasis, code, quotes and rules", () => {
    expect(md("# Mix notes")).toBe("<h1>Mix notes</h1>");
    expect(md("### Bass ###")).toBe("<h3>Bass</h3>");
    expect(md("**loud** and *soft*, ~~old~~ `eq 3k`")).toBe("<p><strong>loud</strong> and <em>soft</em>, <del>old</del> <code>eq 3k</code></p>");
    expect(md("> listen\n> again")).toBe("<blockquote><p>listen again</p></blockquote>");
    expect(md("a\n\n---\n\nb")).toBe("<p>a</p>\n<hr>\n<p>b</p>");
    expect(md("snake_case_name")).toBe("<p>snake_case_name</p>"); // (not emphasis)
  });

  it("draws lists, checklists and tables", () => {
    expect(md("- kick\n- snare\n  wrapped")).toBe("<ul><li>kick</li><li>snare wrapped</li></ul>");
    expect(md("1. intro\n2. drop")).toBe("<ol><li>intro</li><li>drop</li></ol>");
    expect(md("- [x] vocals\n- [ ] master")).toBe('<ul><li class="task"><input type="checkbox" disabled checked> vocals</li><li class="task"><input type="checkbox" disabled> master</li></ul>');
    expect(md("| Track | Gain |\n|---|---:|\n| Bass | -3 |")).toBe(
      "<table><thead><tr><th>Track</th><th>Gain</th></tr></thead><tbody><tr><td>Bass</td><td>-3</td></tr></tbody></table>");
  });
});
