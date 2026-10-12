/*
MIT License

Copyright (c) 2021, Claudéric Demers

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
import type { KeyboardEvent as ReactKeyboardEvent } from "react";
import {
  KeyboardSensor,
  KeyboardCode,
  defaultCoordinates,
  defaultKeyboardCoordinateGetter,
  getClientRect,
  getScrollableAncestors,
  type DraggableNode,
  type KeyboardSensorOptions,
  type SensorProps,
} from "@dnd-kit/core";
import {
  add,
  subtract,
  getOwnerDocument,
  getWindow,
  isKeyboardEvent,
  isHTMLElement,
  type Coordinates,
} from "@dnd-kit/utilities";

/** OutlineKeyboardSensorOptions registers cancellation with the owning outline's lifecycle. */
export interface OutlineKeyboardSensorOptions extends KeyboardSensorOptions {
  register: (cancel: () => void) => () => void;
}

const codes = {
  start: [KeyboardCode.Space, KeyboardCode.Enter],
  end: [KeyboardCode.Space, KeyboardCode.Enter, KeyboardCode.Tab],
  cancel: [KeyboardCode.Esc],
};

function scrollFrame(element: Element) {
  const owner = getOwnerDocument(element);
  const view = getWindow(element);
  const documentScroller = element === owner.scrollingElement;
  const rect = documentScroller
    ? { top: 0, left: 0, right: view.innerWidth, bottom: view.innerHeight }
    : element.getBoundingClientRect();
  const width = documentScroller ? view.innerWidth : element.clientWidth;
  const height = documentScroller ? view.innerHeight : element.clientHeight;
  return {
    ...rect,
    width,
    height,
    maxX: element.scrollWidth - width,
    maxY: element.scrollHeight - height,
  };
}

function scrollAxis(
  element: Element,
  axis: "x" | "y",
  direction: string,
  delta: Coordinates,
  frame: ReturnType<typeof scrollFrame>,
  behavior: ScrollBehavior,
): Coordinates | null {
  const horizontal = axis === "x";
  const forward = direction === (horizontal ? KeyboardCode.Right : KeyboardCode.Down);
  const position = horizontal ? element.scrollLeft : element.scrollTop;
  const maximum = horizontal ? frame.maxX : frame.maxY;
  const proposed = position + delta[axis];
  const reachable = forward ? proposed <= maximum : proposed >= 0;
  const property = horizontal ? "left" : "top";
  if (reachable && delta[horizontal ? "y" : "x"] === 0) {
    element.scrollTo({ [property]: proposed, behavior });
    return null;
  }
  const boundary = forward ? maximum : 0;
  const adjustment = position - (reachable ? proposed : boundary);
  if (adjustment) element.scrollBy({ [property]: -adjustment, behavior });
  return horizontal ? { x: adjustment, y: 0 } : { x: 0, y: adjustment };
}

function canScroll(
  element: Element,
  axis: "x" | "y",
  direction: string,
  maximum: number,
) {
  const horizontal = axis === "x";
  const position = horizontal ? element.scrollLeft : element.scrollTop;
  const forward = horizontal ? KeyboardCode.Right : KeyboardCode.Down;
  const backward = horizontal ? KeyboardCode.Left : KeyboardCode.Up;
  return (
    (direction === forward && position < maximum) ||
    (direction === backward && position > 0)
  );
}

function scrollAdjustment(
  event: KeyboardEvent,
  next: Coordinates,
  current: Coordinates,
  ancestors: Element[],
  behavior: ScrollBehavior,
): Coordinates | null {
  const delta = subtract(next, current);
  const direction = event.code;
  for (const element of ancestors) {
    const frame = scrollFrame(element);
    const right = direction === KeyboardCode.Right;
    const down = direction === KeyboardCode.Down;
    const x = Math.min(
      right ? frame.right - frame.width / 2 : frame.right,
      Math.max(right ? frame.left : frame.left + frame.width / 2, next.x),
    );
    const y = Math.min(
      down ? frame.bottom - frame.height / 2 : frame.bottom,
      Math.max(down ? frame.top : frame.top + frame.height / 2, next.y),
    );
    const canX = canScroll(element, "x", direction, frame.maxX);
    const canY = canScroll(element, "y", direction, frame.maxY);
    if (canX && x !== next.x)
      return scrollAxis(element, "x", direction, delta, frame, behavior);
    if (canY && y !== next.y)
      return scrollAxis(element, "y", direction, delta, frame, behavior);
  }
  return defaultCoordinates;
}

function bringIntoView(node: HTMLElement) {
  if (getScrollableAncestors(node, 1).length === 0) return;
  const rect = getClientRect(node);
  const view = getWindow(node);
  if (
    rect.bottom <= 0 ||
    rect.right <= 0 ||
    rect.top >= view.innerHeight ||
    rect.left >= view.innerWidth
  ) {
    node.scrollIntoView({ block: "center", inline: "center" });
  }
}

/** OutlineKeyboardSensor handles subsequent keys synchronously and ignores only its initiating event. */
export class OutlineKeyboardSensor {
  autoScrollEnabled = false;
  private live = true;
  private reference: Coordinates | undefined;
  private readonly disposers: (() => void)[] = [];
  private readonly node: HTMLElement;
  private readonly activator: EventTarget | null;
  private readonly owner: Document;

  static readonly activators = KeyboardSensor.activators.map(
    ({ eventName, handler }) => ({
      eventName,
      handler: (
        event: ReactKeyboardEvent<HTMLElement>,
        options: OutlineKeyboardSensorOptions,
        context: { active: DraggableNode },
      ) => {
        const target = event.currentTarget;
        if (
          event.target !== target ||
          !isHTMLElement(target) ||
          target.isContentEditable ||
          target.matches("input,textarea,select")
        )
          return false;
        return handler(event, options, context);
      },
    }),
  );

  private readonly props: SensorProps<OutlineKeyboardSensorOptions>;

  constructor(props: SensorProps<OutlineKeyboardSensorOptions>) {
    this.props = props;
    const node = props.activeNode.node.current;
    if (!node) throw new Error("Outline keyboard activation requires a mounted node");
    this.node = node;
    this.activator = props.event.target;
    this.owner = getOwnerDocument(node);
    const view = getWindow(node);
    try {
      this.disposers.push(props.options.register(() => this.finish(true)));
      this.listen(this.owner, "keydown", this.keyDown);
      this.listen(this.owner, "focusin", this.focusChanged);
      this.listen(view, "resize", this.cancel);
      this.listen(view, "visibilitychange", this.cancel);
      this.listen(this.owner, "visibilitychange", this.cancel);
      const observer = new view.MutationObserver(() => {
        if (!this.node.isConnected) this.finish(true);
      });
      this.disposers.push(() => observer.disconnect());
      observer.observe(this.owner.documentElement, { childList: true, subtree: true });
      bringIntoView(node);
      props.onStart(defaultCoordinates);
    } catch (error) {
      this.detach();
      throw error;
    }
  }

  private listen(target: EventTarget, type: string, listener: EventListener) {
    this.disposers.push(() => target.removeEventListener(type, listener));
    target.addEventListener(type, listener);
  }

  private readonly cancel = () => this.finish(true);

  private readonly focusChanged = (event: Event) => {
    if (event.target !== this.activator) this.finish(true);
  };

  private readonly keyDown = (event: Event) => {
    if (!this.live || event === this.props.event || !isKeyboardEvent(event)) return;
    if (
      !this.node.isConnected ||
      event.target !== this.activator ||
      this.owner.activeElement !== this.activator
    ) {
      this.finish(true);
      return;
    }
    const keyboardCodes = this.props.options.keyboardCodes ?? codes;
    if (event.code === KeyboardCode.Tab) {
      this.finish(true);
      return;
    }
    if (keyboardCodes.end.includes(event.code)) {
      event.preventDefault();
      this.finish(false);
      return;
    }
    if (keyboardCodes.cancel.includes(event.code)) {
      event.preventDefault();
      this.finish(true);
      return;
    }
    this.move(event);
  };

  private move(event: KeyboardEvent) {
    const { active, context, options } = this.props;
    const { collisionRect, scrollableAncestors } = context.current;
    const current = collisionRect
      ? { x: collisionRect.left, y: collisionRect.top }
      : defaultCoordinates;
    this.reference ??= current;
    const getter = options.coordinateGetter ?? defaultKeyboardCoordinateGetter;
    const next = getter(event, {
      active,
      context: context.current,
      currentCoordinates: current,
    });
    if (!next || !this.live) return;
    event.preventDefault();
    const scroll = scrollAdjustment(
      event,
      next,
      current,
      scrollableAncestors,
      options.scrollBehavior ?? "smooth",
    );
    if (scroll !== null && this.live)
      this.props.onMove(add(subtract(next, this.reference), scroll));
  }

  private finish(cancel: boolean) {
    if (!this.live) return;
    this.detach();
    if (cancel) this.props.onCancel();
    else this.props.onEnd();
  }

  private detach() {
    if (!this.live) return;
    this.live = false;
    for (const dispose of this.disposers.splice(0).reverse()) dispose();
  }
}
