/**
 * layoutTree.ts — pure helpers for the VS Code-style split layout.
 *
 * The layout is a binary-ish tree. A node is either:
 *   - a **leaf**: holds the session tabs of one editor pane
 *   - a **split**: an internal node with a direction (row = side by side,
 *     column = stacked) and ≥ 2 children
 *
 * All functions here are **pure**: they take a tree and return a new tree,
 * never mutating the input. This keeps React renders predictable and makes
 * the layout round-trip through the URL trivially (just JSON.stringify).
 *
 * Invariants maintained by every helper:
 *   - A split always has ≥ 2 children (an orphan split is collapsed to its
 *     only child on the way out).
 *   - The root is never null — a chat with no sessions is a single empty leaf.
 *   - Leaf ids are unique.
 */

export type SplitDirection = "row" | "column";

export interface LeafNode {
  kind: "leaf";
  id: string;
  tabs: string[];
  active: string | null;
}

export interface SplitNode {
  kind: "split";
  id: string;
  direction: SplitDirection;
  children: LayoutNode[];
}

export type LayoutNode = LeafNode | SplitNode;

// ── Leaf id generator (stable enough; collisions break nothing functional) ──
let _leafSeq = 0;
export function newLeafId(): string {
  _leafSeq += 1;
  return `leaf-${Date.now().toString(36)}-${_leafSeq}`;
}

/** A fresh empty leaf, used as the default root. */
export function createEmptyLeaf(id: string = newLeafId()): LeafNode {
  return { kind: "leaf", id, tabs: [], active: null };
}

// ── Traversal ───────────────────────────────────────────────────────────────

/** Find a leaf node by id, anywhere in the tree. Returns null if missing. */
export function findLeaf(
  tree: LayoutNode,
  leafId: string,
): LeafNode | null {
  if (tree.kind === "leaf") return tree.id === leafId ? tree : null;
  for (const child of tree.children) {
    const found = findLeaf(child, leafId);
    if (found) return found;
  }
  return null;
}

/** Collect every leaf in the tree, in visual (depth-first) order. */
export function collectLeaves(tree: LayoutNode): LeafNode[] {
  if (tree.kind === "leaf") return [tree];
  return tree.children.flatMap(collectLeaves);
}

/** First leaf in visual order (handy for default focus after structural ops). */
export function firstLeaf(tree: LayoutNode): LeafNode {
  if (tree.kind === "leaf") return tree;
  return firstLeaf(tree.children[0]);
}

// ── Mutation helpers (pure) ─────────────────────────────────────────────────

/**
 * Return a new tree with the leaf `leafId` replaced by `patch`.
 * The merge is shallow (Object.assign on the leaf). Safe for the common case
 * of updating `active` or `tabs`.
 */
export function updateLeaf(
  tree: LayoutNode,
  leafId: string,
  patch: Partial<Pick<LeafNode, "tabs" | "active">>,
): LayoutNode {
  if (tree.kind === "leaf") {
    return tree.id === leafId ? { ...tree, ...patch } : tree;
  }
  return {
    ...tree,
    children: tree.children.map((c) => updateLeaf(c, leafId, patch)),
  };
}

/**
 * Remove a leaf from the tree. If its parent ends up with < 2 children the
 * parent collapses: a single remaining child replaces the parent (promoting
 * it one level up). The root is never removed — if you delete the only leaf,
 * it is replaced by an empty leaf.
 */
export function removeLeaf(
  tree: LayoutNode,
  leafId: string,
): LayoutNode {
  if (tree.kind === "leaf") {
    // Root leaf: replace with an empty leaf rather than vanishing.
    return tree.id === leafId ? createEmptyLeaf() : tree;
  }
  const next = tree.children
    .filter((c) => !(c.kind === "leaf" && c.id === leafId))
    .map((c) => (c.kind === "split" ? removeLeaf(c, leafId) : c));

  // After removing a nested leaf, a child split may have collapsed to one —
  // unwrap it so we don't nest pointlessly.
  const unwrapped = next.flatMap((c) =>
    c.kind === "split" && c.children.length === 1 ? c.children : [c],
  );

  if (unwrapped.length === 0) return createEmptyLeaf();
  if (unwrapped.length === 1) return unwrapped[0];
  return { ...tree, children: unwrapped };
}

/**
 * Split `leafId` by inserting `newLeaf` after it in the given direction.
 *
 * - If the leaf's parent is a split of the **same** direction, `newLeaf` is
 *   inserted as the next sibling (right of / below the target leaf).
 * - Otherwise the leaf is wrapped in a new split of the requested direction
 *   containing `[leaf, newLeaf]`.
 *
 * Returns the resulting tree AND the id of the newly inserted leaf so the
 * caller can focus it.
 */
export function splitLeaf(
  tree: LayoutNode,
  leafId: string,
  direction: SplitDirection,
  newLeaf: LeafNode,
): { tree: LayoutNode; newLeafId: string } {
  function recurse(node: LayoutNode): LayoutNode {
    if (node.kind === "leaf") {
      if (node.id !== leafId) return node;
      // Wrap the leaf into a fresh split of the requested direction.
      return {
        kind: "split",
        id: `split-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
        direction,
        children: [node, newLeaf],
      };
    }
    // Split node: if same direction, insert sibling next to the target leaf.
    if (node.direction === direction) {
      const children: LayoutNode[] = [];
      let inserted = false;
      for (const child of node.children) {
        children.push(recurse(child));
        // When the child WAS the target leaf (or became a wrapped split that
        // ends in the target), drop the new sibling right after it.
        if (!inserted && containsLeaf(child, leafId)) {
          children.push(newLeaf);
          inserted = true;
        }
      }
      return { ...node, children };
    }
    // Different direction: recurse and let the leaf wrap itself.
    return { ...node, children: node.children.map(recurse) };
  }

  return { tree: recurse(tree), newLeafId: newLeaf.id };
}

/** Does `tree` contain a leaf with `leafId`? */
function containsLeaf(tree: LayoutNode, leafId: string): boolean {
  return findLeaf(tree, leafId) !== null;
}

/**
 * Move a session tab from `fromLeafId` to `toLeafId`, inserting at
 * `insertIndex` (or appending). The source leaf removes the tab; the target
 * leaf activates it. If the source leaf is left empty, it is removed from the
 * tree (its parent split collapses) — so a cross-pane drag never leaves a
 * dead "No session open" pane behind.
 */
export function moveTabBetweenLeaves(
  tree: LayoutNode,
  sessionId: string,
  fromLeafId: string,
  toLeafId: string,
  insertIndex?: number,
): LayoutNode {
  const from = findLeaf(tree, fromLeafId);
  const to = findLeaf(tree, toLeafId);
  if (!from || !to) return tree;

  // ── Same-leaf reorder: remove-then-insert on ONE array, so the second
  // updateLeaf doesn't clobber the first with a stale snapshot. The
  // insertIndex is relative to the DOM (which still shows the dragged tab),
  // so if the dragged tab sat before the target slot we must decrement by 1.
  if (fromLeafId === toLeafId) {
    const tabs = [...from.tabs];
    const i = tabs.indexOf(sessionId);
    if (i === -1) return tree;
    tabs.splice(i, 1);
    let idx = insertIndex !== undefined ? Math.min(insertIndex, tabs.length) : tabs.length;
    if (insertIndex !== undefined && i < insertIndex) idx = Math.max(0, idx - 1);
    tabs.splice(idx, 0, sessionId);
    return updateLeaf(tree, fromLeafId, { tabs, active: sessionId });
  }

  // ── Cross-leaf move.
  let next = tree;

  // Remove from source.
  const fromTabs = from.tabs.filter((t) => t !== sessionId);
  let fromActive = from.active;
  if (from.active === sessionId) {
    const idx = from.tabs.indexOf(sessionId);
    fromActive = fromTabs[Math.min(idx, fromTabs.length - 1)] ?? null;
  }
  next = updateLeaf(next, fromLeafId, {
    tabs: fromTabs,
    active: fromActive,
  });

  // Insert into target (avoid duplicates).
  const toTabs = [...to.tabs];
  if (!toTabs.includes(sessionId)) {
    const idx =
      insertIndex !== undefined ? Math.min(insertIndex, toTabs.length) : toTabs.length;
    toTabs.splice(idx, 0, sessionId);
  }
  next = updateLeaf(next, toLeafId, { tabs: toTabs, active: sessionId });

  // If the source leaf was left empty, remove it so no dead pane lingers.
  if (fromTabs.length === 0) {
    next = removeLeaf(next, fromLeafId);
  }

  return next;
}

/**
 * Close a tab in a leaf. If the leaf is left with no tabs, remove the leaf
 * from the tree and return the id of a surviving leaf to focus (or null if
 * the root was reset to empty).
 */
export function closeTabInLeaf(
  tree: LayoutNode,
  leafId: string,
  sessionId: string,
): { tree: LayoutNode; nextFocusLeafId: string | null } {
  const leaf = findLeaf(tree, leafId);
  if (!leaf) return { tree, nextFocusLeafId: null };

  const nextTabs = leaf.tabs.filter((t) => t !== sessionId);
  if (nextTabs.length > 0) {
    let nextActive = leaf.active;
    if (leaf.active === sessionId) {
      const idx = leaf.tabs.indexOf(sessionId);
      nextActive = nextTabs[Math.min(idx, nextTabs.length - 1)] ?? null;
    }
    return {
      tree: updateLeaf(tree, leafId, { tabs: nextTabs, active: nextActive }),
      nextFocusLeafId: leafId,
    };
  }

  // Leaf is now empty → remove it from the tree, focus a surviving leaf.
  const removed = removeLeaf(tree, leafId);
  const survivor = firstLeaf(removed);
  return {
    tree: removed,
    nextFocusLeafId: survivor.id === leafId ? null : survivor.id,
  };
}

/** Strip a session id from every leaf (used when a session is deleted). */
export function removeSessionFromTree(
  tree: LayoutNode,
  sessionId: string,
): LayoutNode {
  function recurse(node: LayoutNode): LayoutNode {
    if (node.kind === "leaf") {
      if (!node.tabs.includes(sessionId)) return node;
      const tabs = node.tabs.filter((t) => t !== sessionId);
      let active = node.active;
      if (active === sessionId) active = tabs[0] ?? null;
      return { ...node, tabs, active };
    }
    return { ...node, children: node.children.map(recurse) };
  }
  return recurse(tree);
}

/** Does the tree contain any open tabs? (Gates the "show placeholder" branch.) */
export function hasOpenTabs(tree: LayoutNode): boolean {
  return collectLeaves(tree).some((l) => l.tabs.length > 0);
}
