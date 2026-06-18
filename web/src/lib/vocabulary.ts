export const labels = {
  workspace: 'Workspace',
  workspaces: 'Workspaces',
  source: 'Source',
  sources: 'Sources',
  item: 'Item',
  items: 'Items',
  scope: 'Scope',
  identifier: 'Identifier',
  itemPath: 'Item Path',
  sourceStructure: 'Source Structure'
} as const;

export function metadataSourceLabel(source?: string): string {
  return source === 'docs' ? 'Docs' : 'Item';
}
