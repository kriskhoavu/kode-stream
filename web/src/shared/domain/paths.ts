

export function lastPathSegment(path: string): string {
  const normalized = path.replace(/\/+$/, '');
  const separator = normalized.lastIndexOf('/');
  return separator >= 0 ? normalized.slice(separator + 1) : normalized;
}

export function parentDirectoryPath(path: string): string {
  const normalized = path.replace(/\/+$/, '');
  const separator = normalized.lastIndexOf('/');
  return separator >= 0 ? normalized.slice(0, separator) : '';
}
