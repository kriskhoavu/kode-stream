import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { ErrorBoundary } from './components/ErrorBoundary';
import './styles/app.css';

function Root() {
  const [resetKey, setResetKey] = useState(0);
  useEffect(() => {
    const recover = (event: Event) => {
      const path = (event as CustomEvent<string>).detail || '/workstream';
      history.pushState(null, '', path);
      setResetKey((key) => key + 1);
    };
    window.addEventListener('kode-stream:navigate', recover);
    return () => window.removeEventListener('kode-stream:navigate', recover);
  }, []);
  return <ErrorBoundary resetKey={`${location.pathname}${location.search}:${resetKey}`}><App /></ErrorBoundary>;
}

createRoot(document.getElementById('root')!).render(<React.StrictMode><Root /></React.StrictMode>);
