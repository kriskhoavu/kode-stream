import type { FileKind } from '../../lib/types';
import type { ViewerAdapter, ViewerMode } from './types';

const adapterModes: Record<FileKind, { defaultMode: ViewerMode; modes: ViewerMode[] }> = {
  markdown: { defaultMode: 'rendered', modes: ['rendered', 'source'] },
  html: { defaultMode: 'rendered', modes: ['rendered', 'source'] },
  json: { defaultMode: 'structured', modes: ['structured', 'source'] },
  yaml: { defaultMode: 'structured', modes: ['structured', 'source'] },
  code: { defaultMode: 'source', modes: ['source'] },
  text: { defaultMode: 'source', modes: ['source'] },
  image: { defaultMode: 'rendered', modes: ['rendered'] },
  unsupported: { defaultMode: 'source', modes: [] }
};

export function viewerAdapter(kind: FileKind): ViewerAdapter {
  return { kind, ...adapterModes[kind] };
}
