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
  if (source === 'wiki') return 'Wiki';
  if (source === 'docs') return 'Docs';
  return 'Item';
}

export function isDocumentationMetadataSource(source?: string): boolean {
  return source === 'docs' || source === 'wiki';
}
