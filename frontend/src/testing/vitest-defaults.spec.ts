import { describe, it, expect, vi } from 'vitest';

// Guards a runner default the suite now relies on, see .claude/rules/testing.md.
// Vitest 5 clears every mock's calls before each test (`clearMocks: true`);
// vitest 4 did not, and the Angular builder sets neither. A spec written since
// the upgrade may assert call counts on a mock shared across its tests and pass
// only because of this. If a runner config or a vitest major ever turns it off,
// this file fails first and says why.
const shared = vi.fn();

describe('vitest defaults', () => {
  it('records a call in one test', () => {
    shared();
    expect(shared).toHaveBeenCalledTimes(1);
  });

  it('starts the next test with the calls cleared (clearMocks)', () => {
    expect(shared).not.toHaveBeenCalled();
  });
});
