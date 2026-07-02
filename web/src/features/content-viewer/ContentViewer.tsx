import { lazy, memo, Suspense, useEffect, useState } from 'react';
import { viewerAdapter } from './classify';
import { ViewerErrorBoundary } from './components/ViewerErrorBoundary';
import { ViewerToolbar } from './components/ViewerToolbar';
import { richPreviewThresholdBytes } from './types';
import type { ContentViewerProps, ViewerMode } from './types';
import './content-viewer.css';

const MarkdownPreview = lazy(() => import('./renderers/MarkdownPreview').then((module) => ({ default: module.MarkdownPreview })));
const HtmlPreview = lazy(() => import('./renderers/HtmlPreview').then((module) => ({ default: module.HtmlPreview })));
const StructuredDataView = lazy(() => import('./renderers/StructuredDataView').then((module) => ({ default: module.StructuredDataView })));
const SourceCodeView = lazy(() => import('./renderers/SourceCodeView').then((module) => ({ default: module.SourceCodeView })));

export const ContentViewer = memo(function ContentViewer({ file, content, compact = false }: ContentViewerProps) {
  const adapter = viewerAdapter(file.kind);
  const [mode, setMode] = useState<ViewerMode>(adapter.defaultMode);
  const large = file.kind !== 'image' && file.sizeBytes > richPreviewThresholdBytes;

  useEffect(() => {
    setMode(viewerAdapter(file.kind).defaultMode);
  }, [file.id, file.kind]);

  const showRichPreview = mode !== 'source' && !large;

  return (
    <section className={`content-viewer ${compact ? 'compact' : ''}`} data-file-kind={file.kind}>
      <ViewerToolbar modes={adapter.modes} mode={mode} onChange={setMode} />
      <ViewerErrorBoundary key={`${file.id}:${mode}`}>
        <Suspense fallback={<div className="viewer-loading">Loading preview...</div>}>
          {file.kind === 'unsupported' ? (
            <div className="viewer-empty">
              <strong>This file cannot be displayed as text.</strong>
              <span>{file.path}</span>
            </div>
          ) : !showRichPreview && mode !== 'source' ? (
            <div className="viewer-empty">
              <strong>Rich preview is paused for this large file.</strong>
              <button type="button" className="secondary" onClick={() => setMode('source')}>Open source</button>
            </div>
          ) : file.kind === 'image' ? (
            <div className="content-viewer-image"><img src={content} alt={file.path} /></div>
          ) : mode === 'source' ? (
            <SourceCodeView content={content} language={file.language} truncated={file.truncated} />
          ) : file.kind === 'markdown' ? (
            <MarkdownPreview content={content} />
          ) : file.kind === 'html' ? (
            <HtmlPreview content={content} />
          ) : file.kind === 'json' || file.kind === 'yaml' ? (
            <StructuredDataView content={content} language={file.kind} />
          ) : (
            <SourceCodeView content={content} language={file.language} truncated={file.truncated} />
          )}
        </Suspense>
      </ViewerErrorBoundary>
    </section>
  );
});
