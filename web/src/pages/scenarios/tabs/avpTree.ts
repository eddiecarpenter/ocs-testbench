/**
 * Pure helpers for the Frame tab's AVP tree.
 *
 * Engine-managed AVPs are surfaced read-only. Adding/editing/removing
 * non-managed AVPs runs through these helpers so the call site stays
 * declarative.
 */
import type { AvpNode } from '../types';

/**
 * Returns true for AVPs the engine injects at send time and that the
 * user must NOT touch even for value-reference editing. These are NOT
 * stored in avpTree — they are shown only as informational system rows
 * in the Frame tab UI.
 *
 * Everything else (including Origin-Host, Service-Information, etc.) is
 * stored in avpTree, marked `locked: true` by the defaults layer, and
 * handled by `isLockedAvp` / `isLockedGroup` below.
 */
export const PURE_ENGINE_AVPS: readonly string[] = [
  'Multiple-Services-Indicator',
  'CC-Request-Type',
  'CC-Request-Number',
];

/** @deprecated Use isLockedAvp / isLockedGroup instead. */
export function isManagedAvp(node: AvpNode): boolean {
  return PURE_ENGINE_AVPS.includes(node.name);
}

/**
 * Locked-leaf: cannot delete, CAN edit valueRef.
 * Locked-group: cannot delete, cannot edit, expand/collapse only —
 * but children's leaf values can still be edited.
 */
export function isLockedAvp(node: AvpNode): boolean {
  return node.locked === true;
}

export function isLockedGroup(node: AvpNode): boolean {
  return node.locked === true && Array.isArray(node.children);
}

/** Address into the tree — array of indices, top-down. */
export type AvpPath = number[];

export function getNodeAt(
  tree: AvpNode[],
  path: AvpPath,
): AvpNode | null {
  if (path.length === 0) return null;
  let nodes = tree;
  let cur: AvpNode | null = null;
  for (const i of path) {
    if (i < 0 || i >= nodes.length) return null;
    cur = nodes[i];
    nodes = cur.children ?? [];
  }
  return cur;
}

function clone(tree: AvpNode[]): AvpNode[] {
  return structuredClone(tree);
}

/** Replace the node at `path` with `replacement`. */
export function setNodeAt(
  tree: AvpNode[],
  path: AvpPath,
  replacement: AvpNode,
): AvpNode[] {
  if (path.length === 0) return clone(tree);
  const next = clone(tree);
  let parentChildren: AvpNode[] = next;
  for (let i = 0; i < path.length - 1; i += 1) {
    const idx = path[i];
    const cur = parentChildren[idx];
    cur.children = cur.children ? [...cur.children] : [];
    parentChildren = cur.children;
  }
  parentChildren[path[path.length - 1]] = replacement;
  return next;
}

export function removeNodeAt(
  tree: AvpNode[],
  path: AvpPath,
): AvpNode[] {
  if (path.length === 0) return clone(tree);
  const next = clone(tree);
  let parentChildren: AvpNode[] = next;
  for (let i = 0; i < path.length - 1; i += 1) {
    const idx = path[i];
    const cur = parentChildren[idx];
    cur.children = cur.children ? [...cur.children] : [];
    parentChildren = cur.children;
  }
  parentChildren.splice(path[path.length - 1], 1);
  return next;
}

export function addChildAt(
  tree: AvpNode[],
  path: AvpPath,
  child: AvpNode,
): AvpNode[] {
  if (path.length === 0) return [...clone(tree), child];
  const next = clone(tree);
  let parentChildren: AvpNode[] = next;
  for (let i = 0; i < path.length - 1; i += 1) {
    const idx = path[i];
    const cur = parentChildren[idx];
    cur.children = cur.children ? [...cur.children] : [];
    parentChildren = cur.children;
  }
  const target = parentChildren[path[path.length - 1]];
  target.children = [...(target.children ?? []), child];
  return next;
}

// ---------------------------------------------------------------------------
// Moving nodes around the tree.
//
// Each move returns the new tree AND the moved node's new path so the caller
// can keep the selection on the node it just moved. When a move is not allowed
// the tree and path are returned unchanged — callers should disable the control
// via the matching can* predicate rather than rely on the no-op.
// ---------------------------------------------------------------------------

export interface MoveResult {
  tree: AvpNode[];
  path: AvpPath;
}

/** Returns the sibling array that contains the node at `path`, or null. */
function siblingsAt(tree: AvpNode[], path: AvpPath): AvpNode[] | null {
  if (path.length === 0) return null;
  let nodes = tree;
  for (let i = 0; i < path.length - 1; i += 1) {
    const cur = nodes[path[i]];
    if (!cur || !cur.children) return null;
    nodes = cur.children;
  }
  return nodes;
}

/** The parent node of `path`, or null when the node is at the root level. */
function parentAt(tree: AvpNode[], path: AvpPath): AvpNode | null {
  if (path.length < 2) return null;
  return getNodeAt(tree, path.slice(0, -1));
}

function isGroupNode(node: AvpNode): boolean {
  return Array.isArray(node.children);
}

/** A locked parent's structure is fixed — its children may not be reordered. */
function parentLocked(tree: AvpNode[], path: AvpPath): boolean {
  return parentAt(tree, path)?.locked === true;
}

export function canMoveUp(tree: AvpNode[], path: AvpPath): boolean {
  return (
    path.length > 0 &&
    path[path.length - 1] > 0 &&
    !parentLocked(tree, path)
  );
}

export function canMoveDown(tree: AvpNode[], path: AvpPath): boolean {
  const siblings = siblingsAt(tree, path);
  if (!siblings) return false;
  return (
    path[path.length - 1] < siblings.length - 1 && !parentLocked(tree, path)
  );
}

export function canIndent(tree: AvpNode[], path: AvpPath): boolean {
  const siblings = siblingsAt(tree, path);
  const i = path[path.length - 1];
  if (!siblings || i === 0 || parentLocked(tree, path)) return false;
  const prev = siblings[i - 1];
  // Can only nest into a preceding sibling that is itself an (unlocked) group.
  return isGroupNode(prev) && prev.locked !== true;
}

export function canOutdent(tree: AvpNode[], path: AvpPath): boolean {
  // Must be nested at least one level, and its parent must be editable.
  return path.length >= 2 && !parentLocked(tree, path);
}

/** Swap the node with its previous sibling. */
export function moveUp(tree: AvpNode[], path: AvpPath): MoveResult {
  if (!canMoveUp(tree, path)) return { tree, path };
  const next = clone(tree);
  const siblings = siblingsAt(next, path)!;
  const i = path[path.length - 1];
  [siblings[i - 1], siblings[i]] = [siblings[i], siblings[i - 1]];
  return { tree: next, path: [...path.slice(0, -1), i - 1] };
}

/** Swap the node with its next sibling. */
export function moveDown(tree: AvpNode[], path: AvpPath): MoveResult {
  if (!canMoveDown(tree, path)) return { tree, path };
  const next = clone(tree);
  const siblings = siblingsAt(next, path)!;
  const i = path[path.length - 1];
  [siblings[i + 1], siblings[i]] = [siblings[i], siblings[i + 1]];
  return { tree: next, path: [...path.slice(0, -1), i + 1] };
}

/** Nest the node as the last child of its preceding sibling. */
export function indent(tree: AvpNode[], path: AvpPath): MoveResult {
  if (!canIndent(tree, path)) return { tree, path };
  const next = clone(tree);
  const siblings = siblingsAt(next, path)!;
  const i = path[path.length - 1];
  const [node] = siblings.splice(i, 1);
  const prev = siblings[i - 1];
  prev.children = prev.children ?? [];
  const newIdx = prev.children.length;
  prev.children.push(node);
  return { tree: next, path: [...path.slice(0, -1), i - 1, newIdx] };
}

/** Pop the node up one level, placing it right after its former parent. */
export function outdent(tree: AvpNode[], path: AvpPath): MoveResult {
  if (!canOutdent(tree, path)) return { tree, path };
  const next = clone(tree);
  const parentPath = path.slice(0, -1);
  const parentIdx = parentPath[parentPath.length - 1];
  const grandparentChildren = siblingsAt(next, parentPath)!;
  const siblings = siblingsAt(next, path)!;
  const i = path[path.length - 1];
  const [node] = siblings.splice(i, 1);
  grandparentChildren.splice(parentIdx + 1, 0, node);
  return { tree: next, path: [...parentPath.slice(0, -1), parentIdx + 1] };
}
