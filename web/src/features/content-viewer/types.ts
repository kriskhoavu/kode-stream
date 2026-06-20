import type { FileContent, FileKind } from '../../lib/types';

export type ViewerMode = 'rendered' | 'structured' | 'source';

export interface ContentViewerProps {
  file: FileContent;
  content: string;
  compact?: boolean;
}

export interface ViewerAdapter {
  kind: FileKind;
  defaultMode: ViewerMode;
  modes: ViewerMode[];
}
