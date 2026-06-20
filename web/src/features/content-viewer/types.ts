import type { FileContent, FileKind } from '../../lib/types';

export const richPreviewThresholdBytes = 1 << 20;

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
