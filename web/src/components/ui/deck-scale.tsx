import * as React from "react";

const DeckScaleContext = React.createContext(false);

/**
 * DeckScale is the root of a surface built to the design deck. It renders the
 * element that carries `data-scale="deck"`, which the primitives' deck
 * geometry reads, and tells content that portals out of it, such as a select's
 * list or a menu, to carry the attribute too: a portal keeps its React
 * ancestors but not its DOM ones.
 */
function DeckScale(props: React.ComponentProps<"div">) {
  return (
    <DeckScaleContext value={true}>
      <div data-scale="deck" {...props} />
    </DeckScaleContext>
  );
}

/** useDeckScale reports whether the caller renders under a DeckScale root, portals included. */
function useDeckScale(): boolean {
  return React.useContext(DeckScaleContext);
}

export { DeckScale, useDeckScale };
