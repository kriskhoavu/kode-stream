export type DiffLine = { type: 'context' | 'add' | 'delete' | 'meta'; text: string; oldLine?: number; newLine?: number };
export type DiffFile = { path: string; oldPath?: string; lines: DiffLine[]; additions: number; deletions: number };

export function parseGitDiff(diff: string): DiffFile[] {
  const files: DiffFile[] = [];
  let current: DiffFile | null = null;
  let oldLine = 0;
  let newLine = 0;
  for (const rawLine of diff.split('\n')) {
    if (rawLine.startsWith('diff --git ')) {
      const match = rawLine.match(/^diff --git a\/(.+?) b\/(.+)$/);
      current = {
        oldPath: match?.[1],
        path: match?.[2] ?? match?.[1] ?? rawLine.replace(/^diff --git\s+/, ''),
        lines: [],
        additions: 0,
        deletions: 0
      };
      files.push(current);
      oldLine = 0;
      newLine = 0;
      continue;
    }
    if (!current) continue;
    if (rawLine.startsWith('--- ')) {
      const oldPath = rawLine.replace(/^---\s+a\//, '').replace(/^---\s+/, '');
      if (oldPath !== '/dev/null') current.oldPath = oldPath;
      continue;
    }
    if (rawLine.startsWith('+++ ')) {
      const path = rawLine.replace(/^\+\+\+\s+b\//, '').replace(/^\+\+\+\s+/, '');
      if (path !== '/dev/null') current.path = path;
      continue;
    }
    if (rawLine.startsWith('@@')) {
      const match = rawLine.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$/);
      oldLine = match ? Number(match[1]) : 0;
      newLine = match ? Number(match[2]) : 0;
      current.lines.push({ type: 'meta', text: rawLine });
      continue;
    }
    if (rawLine.startsWith('+')) {
      current.additions += 1;
      current.lines.push({ type: 'add', text: rawLine.slice(1), newLine });
      newLine += 1;
      continue;
    }
    if (rawLine.startsWith('-')) {
      current.deletions += 1;
      current.lines.push({ type: 'delete', text: rawLine.slice(1), oldLine });
      oldLine += 1;
      continue;
    }
    if (rawLine.startsWith('index ') || rawLine.startsWith('new file ') || rawLine.startsWith('deleted file ')) {
      current.lines.push({ type: 'meta', text: rawLine });
      continue;
    }
    current.lines.push({ type: 'context', text: rawLine.startsWith(' ') ? rawLine.slice(1) : rawLine, oldLine, newLine });
    oldLine += 1;
    newLine += 1;
  }
  return files.filter((item) => item.lines.length > 0 || item.additions > 0 || item.deletions > 0);
}
