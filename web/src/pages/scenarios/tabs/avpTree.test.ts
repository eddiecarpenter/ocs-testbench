import { describe, expect, it } from 'vitest';

import type { AvpNode } from '../types';
import {
  canIndent,
  canMoveDown,
  canMoveUp,
  canOutdent,
  getNodeAt,
  indent,
  moveDown,
  moveUp,
  outdent,
} from './avpTree';

// Mirrors the "USSD2 Charge Event" shape: an empty group container plus a
// nested leaf we want to lift to the top level.
function sampleTree(): AvpNode[] {
  return [
    { name: 'Subscription-Id', code: 443, valueRef: 'MSISDN' },
    {
      name: 'Service-Information',
      code: 873,
      children: [
        {
          name: 'USSD-Information',
          code: 20600,
          children: [
            { name: 'USSD-Detail', code: 20654, valueRef: 'USSD_DETAIL' },
          ],
        },
      ],
    },
  ];
}

describe('avpTree move helpers', () => {
  it('moveUp/moveDown swap adjacent siblings and follow the node', () => {
    const tree = sampleTree();
    const down = moveDown(tree, [0]);
    expect(down.path).toEqual([1]);
    expect(getNodeAt(down.tree, [1])?.name).toBe('Subscription-Id');
    // Original tree is untouched (immutability).
    expect(getNodeAt(tree, [0])?.name).toBe('Subscription-Id');

    const up = moveUp(down.tree, [1]);
    expect(up.path).toEqual([0]);
    expect(getNodeAt(up.tree, [0])?.name).toBe('Subscription-Id');
  });

  it('outdent lifts a nested node to be the next sibling of its parent', () => {
    const tree = sampleTree();
    // USSD-Detail lives at [1,0,0]; outdent once → sibling of USSD-Information.
    const once = outdent(tree, [1, 0, 0]);
    expect(once.path).toEqual([1, 1]);
    expect(getNodeAt(once.tree, [1, 1])?.name).toBe('USSD-Detail');
    // USSD-Information is now an empty group.
    expect(getNodeAt(once.tree, [1, 0])?.children).toEqual([]);

    // outdent again → top level, sibling of Service-Information.
    const twice = outdent(once.tree, [1, 1]);
    expect(twice.path).toEqual([2]);
    expect(getNodeAt(twice.tree, [2])?.name).toBe('USSD-Detail');
  });

  it('indent nests a node under its preceding sibling group', () => {
    const tree = sampleTree();
    // First lift USSD-Detail to top level so it sits after Service-Information.
    const lifted = outdent(outdent(tree, [1, 0, 0]).tree, [1, 1]);
    expect(getNodeAt(lifted.tree, [2])?.name).toBe('USSD-Detail');
    // Now indent it back under Service-Information (the group at index 1).
    const nested = indent(lifted.tree, [2]);
    expect(nested.path).toEqual([1, 1]);
    expect(getNodeAt(nested.tree, [1, 1])?.name).toBe('USSD-Detail');
  });

  it('boundary predicates gate the controls', () => {
    const tree = sampleTree();
    expect(canMoveUp(tree, [0])).toBe(false); // first sibling
    expect(canMoveDown(tree, [1])).toBe(false); // last sibling
    expect(canOutdent(tree, [0])).toBe(false); // already at root
    expect(canOutdent(tree, [1, 0, 0])).toBe(true); // nested → can pop up
    // USSD-Detail is the only child, no preceding sibling → cannot indent.
    expect(canIndent(tree, [1, 0, 0])).toBe(false);
  });

  it('refuses to reorder children of a locked group', () => {
    const tree: AvpNode[] = [
      {
        name: 'Service-Information',
        code: 873,
        locked: true,
        children: [
          { name: 'A', code: 1, valueRef: '1' },
          { name: 'B', code: 2, valueRef: '2' },
        ],
      },
    ];
    expect(canMoveDown(tree, [0, 0])).toBe(false);
    expect(canOutdent(tree, [0, 0])).toBe(false);
    // move is a no-op when disallowed.
    expect(moveDown(tree, [0, 0]).tree).toBe(tree);
  });
});
