import { useState } from "react";
import {
  DndContext,
  useDraggable,
  useSensor,
  type KeyboardCoordinateGetter,
} from "@dnd-kit/core";
import { OutlineKeyboardSensor } from "@/features/tests/components/OutlineKeyboardSensor";
import { afterEach, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { OutlineTree } from "@/features/tests/components/OutlineTree";
import type { OutlineSection } from "@/features/tests/outline";
import "@/lib/i18n";

const tall: OutlineSection = {
  id: null,
  clientId: "client-tall",
  title: "Tall",
  instructions: null,
  questionIds: [],
};
const middle: OutlineSection = {
  id: "middle",
  clientId: "client-middle",
  title: "Middle",
  instructions: null,
  questionIds: [],
};
const last: OutlineSection = {
  id: "last",
  clientId: "client-last",
  title: "Last",
  instructions: null,
  questionIds: [],
};

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function setup(initial: OutlineSection[]) {
  const changed = vi.fn();
  function Harness() {
    const [sections, setSections] = useState(initial);
    return (
      <OutlineTree
        sections={sections}
        questions={new Map()}
        selectedId={null}
        creating={false}
        onSelect={vi.fn()}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onChange={(next) => {
          changed(next);
          setSections(next);
        }}
      />
    );
  }
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const roots = [
      ...document.querySelectorAll('button[aria-roledescription="sortable"]'),
    ]
      .map((grip) => grip.parentElement?.parentElement)
      .filter((node): node is HTMLElement => node instanceof HTMLElement);
    const section = roots.find((node) => node === this || node.contains(this));
    const index = roots.indexOf(section!);
    const top = roots
      .slice(0, Math.max(0, index))
      .reduce((sum, node) => sum + (node.textContent?.includes("Tall") ? 234 : 40), 0);
    const isTall = section?.textContent?.includes("Tall");
    const sectionHeight = isTall ? 234 : 40;
    const height = section?.isSameNode(this) ? sectionHeight : 32;
    const width = isTall ? 320 : 280;
    return {
      width,
      height,
      top,
      left: 10,
      right: 10 + width,
      bottom: top + height,
      x: 10,
      y: top,
      toJSON: () => ({}),
    };
  });
  vi.useFakeTimers();
  return { ...render(<Harness />), changed };
}

function key(grip: Element, code: string) {
  const view = grip.ownerDocument.defaultView!;
  const event = new view.KeyboardEvent("keydown", {
    key: code === "Space" ? " " : code,
    code,
    bubbles: true,
    cancelable: true,
  });
  fireEvent(grip, event);
  fireEvent.keyUp(grip, { key: code === "Space" ? " " : code, code });
  return event.defaultPrevented;
}

function withTree(
  assertions: (fixture: ReturnType<typeof setup> & { grip: HTMLElement }) => void,
) {
  const fixture = setup([tall, middle, last]);
  const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Tall" });
  grip.focus();
  try {
    assertions({ ...fixture, grip });
  } finally {
    act(() => {
      vi.advanceTimersByTime(0);
      fireEvent.keyDown(document, { key: "Escape", code: "Escape" });
    });
    cleanup();
  }
}

function assertVisibleOrder(expected: OutlineSection[]) {
  expect(
    screen
      .getAllByRole("button", { name: /^Kéo để đổi vị trí phần / })
      .map((button) => button.getAttribute("aria-label")),
  ).toEqual(expected.map((section) => `Kéo để đổi vị trí phần ${section.title}`));
}

it.each(["Space", "Enter"])(
  "handles immediate ArrowDown and %s before any timer fires",
  (code) => {
    withTree(({ grip, changed }) => {
      expect(key(grip, code)).toBe(true);
      expect(grip).toHaveAttribute("aria-pressed", "true");
      expect(key(grip, "ArrowDown")).toBe(true);
      expect(key(grip, code)).toBe(true);
      expect(changed).toHaveBeenCalledExactlyOnceWith([middle, tall, last]);
      expect(changed.mock.calls[0]?.[0][1]).toBe(tall);
      assertVisibleOrder([middle, tall, last]);
      expect(grip).not.toHaveAttribute("aria-pressed", "true");
    });
  },
);

it("ends an immediate Escape without changing complete sections", () => {
  withTree(({ grip, changed }) => {
    expect(key(grip, "Space")).toBe(true);
    expect(key(grip, "Escape")).toBe(true);
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    expect(changed).not.toHaveBeenCalled();
    assertVisibleOrder([tall, middle, last]);
  });
});

it("ignores only the actual pickup event while it bubbles, then drops in place without writing", () => {
  withTree(({ grip, changed }) => {
    expect(key(grip, "Space")).toBe(true);
    expect(grip).toHaveAttribute("aria-pressed", "true");
    expect(changed).not.toHaveBeenCalled();
    expect(key(grip, "Space")).toBe(true);
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    expect(changed).not.toHaveBeenCalled();
    assertVisibleOrder([tall, middle, last]);
  });
});

it("allows repeated arrows, clamps at the end, and commits only the final whole section order", () => {
  withTree(({ grip, changed }) => {
    key(grip, "Space");
    expect(key(grip, "ArrowUp")).toBe(true);
    expect(changed).not.toHaveBeenCalled();
    expect(key(grip, "ArrowDown")).toBe(true);
    fireEvent.keyDown(grip, { key: "ArrowDown", code: "ArrowDown", repeat: true });
    expect(key(grip, "ArrowDown")).toBe(true);
    expect(changed).not.toHaveBeenCalled();
    key(grip, "Space");
    expect(changed).toHaveBeenCalledExactlyOnceWith([middle, last, tall]);
    assertVisibleOrder([middle, last, tall]);
  });
});

it("keeps ordinary keys native and cancels Tab without writing or trapping focus", () => {
  withTree(({ grip, changed }) => {
    expect(key(grip, "KeyA")).toBe(false);
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    key(grip, "Space");
    expect(key(grip, "KeyA")).toBe(false);
    expect(grip).toHaveAttribute("aria-pressed", "true");
    key(grip, "ArrowDown");
    expect(key(grip, "Tab")).toBe(false);
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    expect(changed).not.toHaveBeenCalled();
    assertVisibleOrder([tall, middle, last]);
  });
});

it("does not activate from a nested icon, unrelated title button, input or editable child", () => {
  withTree(({ grip, changed }) => {
    const icon = grip.querySelector("svg")!;
    expect(key(icon, "Space")).toBe(false);
    const title = screen.getByRole("button", { name: /^Tall / });
    expect(key(title, "Enter")).toBe(false);
    for (const tag of ["input", "textarea", "div"]) {
      const control = document.createElement(tag);
      if (tag === "div") control.contentEditable = "true";
      grip.append(control);
      expect(key(control, "Space")).toBe(false);
      control.remove();
    }
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    expect(changed).not.toHaveBeenCalled();
  });
});

it("cancels when focus moves to another control without consuming its next Space", () => {
  withTree(({ grip, changed }) => {
    key(grip, "Space");
    key(grip, "ArrowDown");
    const title = screen.getByRole("button", { name: /^Middle / });
    act(() => title.focus());
    expect(grip).not.toHaveAttribute("aria-pressed", "true");
    expect(key(title, "Space")).toBe(false);
    expect(changed).not.toHaveBeenCalled();
    assertVisibleOrder([tall, middle, last]);
  });
});

it.each(["resize", "visibilitychange"])(
  "cancels %s once and leaves later keys native",
  (type) => {
    withTree(({ grip, changed }) => {
      key(grip, "Space");
      key(grip, "ArrowDown");
      fireEvent(window, new Event(type));
      fireEvent(window, new Event(type));
      expect(grip).not.toHaveAttribute("aria-pressed", "true");
      expect(key(grip, "Escape")).toBe(false);
      expect(changed).not.toHaveBeenCalled();
      assertVisibleOrder([tall, middle, last]);
    });
  },
);

it("detaches an active sensor on unmount and a remounted outline owns only its new operation", () => {
  const added = vi.spyOn(document, "addEventListener");
  const removed = vi.spyOn(document, "removeEventListener");
  const first = setup([tall, middle, last]);
  const firstGrip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Tall" });
  firstGrip.focus();
  added.mockClear();
  key(firstGrip, "Space");
  const keyListeners = added.mock.calls
    .filter(([type]) => type === "keydown")
    .map(([, listener]) => listener);
  expect(keyListeners).toHaveLength(1);
  first.unmount();
  expect(
    removed.mock.calls
      .filter(
        ([type, listener]) => type === "keydown" && keyListeners.includes(listener),
      )
      .map(([, listener]) => listener),
  ).toEqual(keyListeners);
  expect(first.changed).not.toHaveBeenCalled();
  withTree(({ grip, changed }) => {
    key(grip, "Space");
    key(grip, "ArrowDown");
    key(grip, "Space");
    expect(changed).toHaveBeenCalledExactlyOnceWith([middle, tall, last]);
    expect(first.changed).not.toHaveBeenCalled();
  });
});

it("cancels an externally removed active node before it can consume another control", async () => {
  const fixture = setup([tall, middle, last]);
  const grip = screen.getByRole("button", { name: "Kéo để đổi vị trí phần Tall" });
  grip.focus();
  key(grip, "Space");
  const wrapper = grip.parentElement!.parentElement!;
  const parent = wrapper.parentElement!;
  const sibling = wrapper.nextSibling;
  await act(async () => {
    wrapper.remove();
  });
  const other = screen.getByRole("button", { name: /^Middle / });
  other.focus();
  expect(key(other, "Space")).toBe(false);
  expect(fixture.changed).not.toHaveBeenCalled();
  parent.insertBefore(wrapper, sibling);
  fixture.unmount();
});

function ScrollDraggable() {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef } = useDraggable({
    id: "scroll-item",
  });
  return (
    <button
      type="button"
      aria-label="Scroll drag"
      ref={(node) => {
        setNodeRef(node);
        setActivatorNodeRef(node);
      }}
      {...attributes}
      {...listeners}
    />
  );
}

function scrollFixture(
  delta: { x: number; y: number },
  max = 400,
  offscreen = false,
  owner = document,
  started: () => void = () => {},
) {
  vi.useFakeTimers();
  const moved = vi.fn(),
    ended = vi.fn(),
    cancelled = vi.fn();
  let dispose: (() => void) | undefined;
  const getter: KeyboardCoordinateGetter = (event, { currentCoordinates }) =>
    ["ArrowDown", "ArrowRight"].includes(event.code)
      ? { x: currentCoordinates.x + delta.x, y: currentCoordinates.y + delta.y }
      : undefined;
  function Harness() {
    const sensor = useSensor(OutlineKeyboardSensor, {
      coordinateGetter: getter,
      scrollBehavior: "auto",
      register: (cancel) => {
        dispose = cancel;
        return () => {
          dispose = undefined;
        };
      },
    });
    return (
      <div data-testid="scroller" style={{ overflow: "auto" }}>
        <DndContext
          sensors={[sensor]}
          onDragStart={started}
          onDragMove={({ delta: movement }) => moved(movement)}
          onDragEnd={ended}
          onDragCancel={cancelled}
        >
          <ScrollDraggable />
        </DndContext>
      </div>
    );
  }
  const mount = owner.createElement("div");
  owner.body.append(mount);
  const fixture = render(<Harness />, { container: mount });
  const scroller = within(mount).getByTestId("scroller");
  const grip = within(mount).getByRole("button", { name: "Scroll drag" });
  Object.defineProperties(scroller, {
    clientHeight: { value: 100, configurable: true },
    clientWidth: { value: 100, configurable: true },
    scrollHeight: { value: max + 100, configurable: true },
    scrollWidth: { value: max + 100, configurable: true },
  });
  vi.spyOn(
    owner.defaultView!.Element.prototype,
    "getBoundingClientRect",
  ).mockImplementation(function (this: Element) {
    const gripTop = offscreen ? 1000 : 20;
    const top = this === grip ? gripTop : 0;
    const left = this === grip ? 20 : 0;
    const size = this === grip ? 30 : 100;
    return {
      top,
      left,
      right: left + size,
      bottom: top + size,
      width: size,
      height: size,
      x: left,
      y: top,
      toJSON: () => ({}),
    };
  });
  const scrollTo = vi.fn(),
    scrollBy = vi.fn();
  Object.defineProperties(scroller, {
    scrollTo: { value: scrollTo, configurable: true },
    scrollBy: { value: scrollBy, configurable: true },
  });
  grip.focus();
  return {
    ...fixture,
    grip,
    scroller,
    scrollTo,
    scrollBy,
    moved,
    ended,
    cancelled,
    dispose: () => {
      act(() => dispose?.());
      fixture.unmount();
    },
  };
}

it.each([
  {
    name: "vertical",
    delta: { x: 0, y: 120 },
    code: "ArrowDown",
    expected: { top: 120, behavior: "auto" },
  },
  {
    name: "horizontal",
    delta: { x: 120, y: 0 },
    code: "ArrowRight",
    expected: { left: 120, behavior: "auto" },
  },
])(
  "preserves $name scroll-only movement before end/cancel",
  ({ delta, code, expected }) => {
    const fixture = scrollFixture(delta);
    try {
      key(fixture.grip, "Space");
      expect(key(fixture.grip, code)).toBe(true);
      expect(fixture.scrollTo).toHaveBeenCalledExactlyOnceWith(expected);
      expect(fixture.scrollBy).not.toHaveBeenCalled();
      expect(fixture.moved).not.toHaveBeenCalled();
      key(fixture.grip, "Escape");
      expect(fixture.cancelled).toHaveBeenCalledOnce();
      expect(fixture.ended).not.toHaveBeenCalled();
    } finally {
      fixture.dispose();
    }
  },
);

it("clamps exhausted scrolling and compensates the real DndContext movement", () => {
  const fixture = scrollFixture({ x: 0, y: 120 }, 80);
  try {
    key(fixture.grip, "Space");
    key(fixture.grip, "ArrowDown");
    expect(fixture.scrollBy).toHaveBeenCalledExactlyOnceWith({
      top: 80,
      behavior: "auto",
    });
    expect(fixture.scrollTo).not.toHaveBeenCalled();
    expect(fixture.moved).toHaveBeenCalledExactlyOnceWith({ x: 0, y: 40 });
    key(fixture.grip, "Escape");
  } finally {
    fixture.dispose();
  }
});

it("brings an initially offscreen draggable into view before starting", () => {
  const fixture = scrollFixture({ x: 0, y: 120 }, 400, true);
  const intoView = vi.spyOn(fixture.grip, "scrollIntoView");
  try {
    key(fixture.grip, "Space");
    expect(intoView).toHaveBeenCalledExactlyOnceWith({
      block: "center",
      inline: "center",
    });
    expect(fixture.grip).toHaveAttribute("aria-pressed", "true");
    key(fixture.grip, "Escape");
  } finally {
    fixture.dispose();
  }
});

it("compensates mixed-axis movement when the requested scroll is reachable", () => {
  const fixture = scrollFixture({ x: 10, y: 120 });
  try {
    key(fixture.grip, "Space");
    key(fixture.grip, "ArrowDown");
    expect(fixture.scrollBy).toHaveBeenCalledExactlyOnceWith({
      top: 120,
      behavior: "auto",
    });
    expect(fixture.moved).toHaveBeenCalledExactlyOnceWith({ x: 10, y: 0 });
    key(fixture.grip, "Escape");
  } finally {
    fixture.dispose();
  }
});

it("registers and detaches on the draggable's actual owner document and window", () => {
  const iframe = document.createElement("iframe");
  document.body.append(iframe);
  const owner = iframe.contentDocument!;
  const view = owner.defaultView!;
  const parentAdded = vi.spyOn(document, "addEventListener");
  const childAdded = vi.spyOn(owner, "addEventListener");
  const childRemoved = vi.spyOn(owner, "removeEventListener");
  const fixture = scrollFixture({ x: 0, y: 20 }, 400, false, owner);
  parentAdded.mockClear();
  childAdded.mockClear();
  try {
    expect(key(fixture.grip, "Space")).toBe(true);
    expect(childAdded.mock.calls.filter(([type]) => type === "keydown")).toHaveLength(
      1,
    );
    expect(parentAdded.mock.calls.filter(([type]) => type === "keydown")).toHaveLength(
      0,
    );
    const listener = childAdded.mock.calls.find(([type]) => type === "keydown")![1];
    fireEvent(view, new view.Event("resize"));
    expect(fixture.cancelled).toHaveBeenCalledOnce();
    expect(
      childRemoved.mock.calls.filter(
        ([type, callback]) => type === "keydown" && callback === listener,
      ),
    ).toHaveLength(1);
    expect(key(fixture.grip, "Escape")).toBe(false);
  } finally {
    fixture.dispose();
    iframe.remove();
  }
});

it("moves a whole shared-context group with immediate keys while retaining members and section identities", async () => {
  const { emptyGroup, newGroupQuestion } =
    await import("@/features/question-groups/model");
  const i18n = (await import("@/lib/i18n")).default;
  const bundle = emptyGroup("Whole group");
  const member = { ...newGroupQuestion(i18n.t), id: "member-first" };
  bundle.questions = [member];
  bundle.group.members = [{ questionId: member.id, optionOrder: "shuffle" }];
  const from: OutlineSection = {
    id: "from",
    clientId: "from-client",
    title: "From",
    instructions: null,
    questionIds: [],
    units: [{ kind: "group", id: bundle.group.id }],
  };
  const to: OutlineSection = {
    id: "to",
    clientId: "to-client",
    title: "To",
    instructions: null,
    questionIds: [],
  };
  const changed = vi.fn();
  function Harness() {
    const [sections, setSections] = useState([from, to]);
    return (
      <OutlineTree
        sections={sections}
        questions={new Map()}
        groups={new Map([[bundle.group.id, bundle]])}
        selectedId={null}
        creating={false}
        onSelect={vi.fn()}
        onCreateQuestion={vi.fn()}
        onPickFromBank={vi.fn()}
        onAddSection={vi.fn()}
        onChange={(next) => {
          changed(next);
          setSections(next);
        }}
      />
    );
  }
  vi.useFakeTimers();
  const fixture = render(<Harness />);
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (
    this: Element,
  ) {
    const section = this.closest("[data-outline-section]");
    const top =
      (section?.getAttribute("data-outline-section") === "to" ? 120 : 0) +
      (this.closest("[data-outline-group]") ? 40 : 0);
    const height = this.parentElement?.hasAttribute("data-outline-section") ? 100 : 32;
    return {
      width: 300,
      height,
      top,
      left: 0,
      right: 300,
      bottom: top + height,
      x: 0,
      y: top,
      toJSON: () => ({}),
    };
  });
  const destination = screen.getByRole("button", { name: /^To / });
  fireEvent.click(destination);
  const grip = screen.getByRole("button", { name: "Kéo nhóm Whole group" });
  grip.focus();
  try {
    expect(key(grip, "Space")).toBe(true);
    expect(key(grip, "ArrowDown")).toBe(true);
    expect(changed).not.toHaveBeenCalled();
    expect(key(grip, "Space")).toBe(true);
    expect(changed).toHaveBeenCalledExactlyOnceWith([
      { ...from, units: [] },
      { ...to, units: [{ kind: "group", id: bundle.group.id }] },
    ]);
    expect(
      fixture.container.querySelectorAll(`[data-outline-group="${bundle.group.id}"]`),
    ).toHaveLength(1);
    expect(destination).toHaveAttribute("aria-expanded", "true");
    expect(bundle.questions).toEqual([member]);
  } finally {
    key(grip, "Escape");
    fixture.unmount();
  }
});

it("removes already installed public listeners when the real start callback fails", () => {
  const failure = new Error("expected public start failure");
  const errors: unknown[] = [];
  const capture = (event: ErrorEvent) => {
    if (event.error === failure) {
      errors.push(event.error);
      event.preventDefault();
    }
  };
  const fixture = scrollFixture({ x: 0, y: 20 }, 400, false, document, () => {
    throw failure;
  });
  const added = vi.spyOn(document, "addEventListener");
  const removed = vi.spyOn(document, "removeEventListener");
  window.addEventListener("error", capture);
  try {
    key(fixture.grip, "Space");
    expect(errors).toEqual([failure]);
    const installed = added.mock.calls.filter(([type]) =>
      ["keydown", "focusin", "visibilitychange"].includes(type),
    );
    expect(installed).toHaveLength(3);
    for (const [type, listener] of installed) {
      expect(
        removed.mock.calls.filter(
          ([removedType, callback]) => type === removedType && callback === listener,
        ),
      ).toHaveLength(1);
    }
    expect(key(fixture.grip, "Escape")).toBe(false);
    expect(fixture.moved).not.toHaveBeenCalled();
    expect(fixture.ended).not.toHaveBeenCalled();
  } finally {
    window.removeEventListener("error", capture);
    fixture.dispose();
  }
});
