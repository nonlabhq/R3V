import { afterAll, afterEach, describe, expect, it, vi } from "vitest";
import { dragSort, type DragSort } from "./dragsort";

// Three items side by side, 100px wide (jsdom lays nothing out).
function list() {
  const ul = document.createElement("ul");
  for (const k of ["a", "b", "c"]) {
    const li = document.createElement("li");
    li.dataset.key = k;
    ul.append(li);
  }
  document.body.append(ul);
  return ul;
}
const press = (el: HTMLElement, clientX: number, o: DragSort, button = 0) => {
  const e = new PointerEvent("pointerdown", { button, clientX });
  Object.defineProperty(e, "currentTarget", { value: el });
  dragSort(e, o);
};
const move = (clientX: number) => window.dispatchEvent(new PointerEvent("pointermove", { buttons: 1, clientX }));

describe("dragSort", () => {
  const spies = [
    vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockImplementation(function (this: HTMLElement) {
      return this.dataset.key ? [...this.parentElement!.children].indexOf(this) * 100 : 0;
    }),
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(100),
  ];
  afterEach(() => { document.body.innerHTML = ""; });
  afterAll(() => spies.forEach((s) => s.mockRestore()));

  it("is a click until the pointer goes a few pixels, then says where the item goes and how far it is from its place", () => {
    const ul = list();
    const a = ul.children[0] as HTMLElement;
    const calls: string[] = [];
    const o: DragSort = {
      axis: "x",
      start: () => calls.push("start"),
      // (what the caller does: draws the new order)
      move: (before) => { calls.push(`before ${before ?? "end"}`); before ? ul.insertBefore(a, ul.querySelector(`[data-key=${before}]`)) : ul.append(a); },
      follow: (d) => calls.push(`follow ${d}`),
      end: (moved) => calls.push(`end ${moved}`),
    };
    press(a, 50, o);
    move(53);
    expect(calls).toEqual([]);
    move(160); // a's middle (110+50) past b's (150)
    expect(calls).toEqual(["start", "before c", "follow 10"]);
    move(300); // past c's: at the end
    expect(calls.slice(3)).toEqual(["before end", "follow 50"]);
    window.dispatchEvent(new PointerEvent("pointerup"));
    expect(calls.at(-1)).toBe("end true");

    // a press let go where it was: a click
    calls.length = 0;
    press(a, 250, o);
    move(252);
    window.dispatchEvent(new PointerEvent("pointerup"));
    expect(calls).toEqual(["end false"]);
    // (a middle or right press is no drag)
    press(a, 250, o, 1);
    move(400);
    window.dispatchEvent(new PointerEvent("pointerup"));
    expect(calls).toEqual(["end false"]);
  });
});
