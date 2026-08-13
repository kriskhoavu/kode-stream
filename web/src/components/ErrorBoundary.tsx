import { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
  children: ReactNode;
  resetKey?: string;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  componentDidMount() { window.addEventListener('popstate', this.resetForNavigation); }
  componentWillUnmount() { window.removeEventListener('popstate', this.resetForNavigation); }

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(error, info.componentStack);
  }

  componentDidUpdate(previous: Props) {
    if (this.state.error && previous.resetKey !== this.props.resetKey) this.setState({ error: null });
  }

  private resetForNavigation = () => {
    if (this.state.error) this.setState({ error: null });
  };

  render() {
    if (this.state.error) {
      return (
        <main className="runtime-error">
          <h1>Kode Stream hit a UI error</h1>
          <p>Something went wrong while rendering this view. You can safely try again or return to the workstream.</p>
          <button onClick={() => this.setState({ error: null })}>Try again</button>
          <button onClick={() => window.dispatchEvent(new CustomEvent('kode-stream:navigate', { detail: '/workstream' }))}>Return to Workstream</button>
        </main>
      );
    }
    return this.props.children;
  }
}
