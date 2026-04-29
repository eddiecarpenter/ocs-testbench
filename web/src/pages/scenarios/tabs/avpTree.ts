/**
 * Pure helpers for the Frame tab's AVP tree.
 *
 * Engine-managed AVPs are surfaced read-only. Adding/editing/removing
 * non-managed AVPs runs through these helpers so the call site stays
 * declarative.
 */
import type { AvpNode } from '../types';

/** Engine-managed AVP names — read-only in the Frame tab (ARCH §8). */
export const ENGINE_MANAGED_AVPS: readonly string[] = [
  'Multiple-Services-Indicator',
  'CC-Request-Type',
  'CC-Request-Number',
  'Session-Id',
];

export function isManagedAvp(node: AvpNode): boolean {
  return ENGINE_MANAGED_AVPS.includes(node.name);
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
