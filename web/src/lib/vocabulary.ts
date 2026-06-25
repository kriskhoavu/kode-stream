export const labels = {
  workspace: 'Workspace',
  workspaces: 'Workspaces',
  source: 'Source',
  sources: 'Sources',
  item: 'Item',
  items: 'Items',
  scope: 'Source',
  identifier: 'Item',
  itemPath: 'Item Path',
  sourceStructure: 'Source Items'
} as const;

export function metadataSourceLabel(source?: string): string {
  return source === 'docs' ? 'Docs' : 'Item';
}
