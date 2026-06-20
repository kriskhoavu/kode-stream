import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';

export class ViewerErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Content viewer failed', error, info);
  }

  render() {
    if (this.state.failed) {
      return (
        <div className="viewer-error" role="alert">
          <strong>This preview could not be rendered.</strong>
          <span>Use Source mode to inspect the file.</span>
        </div>
      );
    }
    return this.props.children;
  }
}
