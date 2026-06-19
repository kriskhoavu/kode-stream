import { describe, expect, it } from 'vitest';
import { parseGitDiff } from '../shared/domain/diff';

describe('parseGitDiff', () => {
  it('parses additions and deletions with line numbers', () => {
    const files = parseGitDiff(`diff --git a/plans/platform/PM-003/README.md b/plans/platform/PM-003/README.md
index 1111111..2222222 100644
--- a/plans/platform/PM-003/README.md
+++ b/plans/platform/PM-003/README.md
@@ -1,3 +1,3 @@
 # PM-003
-Old text
+New text
 Context
`);

    expect(files).toHaveLength(1);
    expect(files[0].path).toBe('plans/platform/PM-003/README.md');
    expect(files[0].additions).toBe(1);
    expect(files[0].deletions).toBe(1);
    expect(files[0].lines.filter((line) => line.type === 'add')).toEqual([{ type: 'add', text: 'New text', newLine: 2 }]);
    expect(files[0].lines.filter((line) => line.type === 'delete')).toEqual([{ type: 'delete', text: 'Old text', oldLine: 2 }]);
  });

  it('preserves rename old and new paths', () => {
    const files = parseGitDiff(`diff --git a/docs/old.md b/docs/new.md
similarity index 80%
rename from docs/old.md
rename to docs/new.md
--- a/docs/old.md
+++ b/docs/new.md
@@ -1 +1 @@
-Old
+New
`);

    expect(files).toHaveLength(1);
    expect(files[0].oldPath).toBe('docs/old.md');
    expect(files[0].path).toBe('docs/new.md');
  });
});
