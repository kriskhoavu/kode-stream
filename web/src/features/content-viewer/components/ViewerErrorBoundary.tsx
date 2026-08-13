import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';

export class ViewerErrorBoundary extends Component<{ children: ReactNode; resetKey?: string }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Content viewer failed', error, info);
  }

  componentDidUpdate(previous: { resetKey?: string }) {
    if (this.state.failed && previous.resetKey !== this.props.resetKey) this.setState({ failed: false });
  }

  render() {
    if (this.state.failed) {
      return (
        <div className="viewer-error" role="alert">
          <strong>This preview could not be rendered safely.</strong>
          <span>Use Source mode to inspect the file.</span>
        </div>
      );
    }
    return this.props.children;
  }
}
