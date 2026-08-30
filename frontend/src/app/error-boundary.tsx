import * as React from 'react';

interface State {
  error: Error | null;
}

/**
 * Last-resort boundary.
 *
 * The 2026-05-04 incident was a runtime error thrown before event handlers
 * were registered: the page rendered, nothing responded, and the version badge
 * stayed on "loading" with no indication that anything had failed. A boundary
 * turns that class of failure into a visible message with a recovery action.
 *
 * It is intentionally free of hooks, translations and design tokens: it has to
 * render even when the providers below it are the thing that broke.
 */
export class RootErrorBoundary extends React.Component<{ children: React.ReactNode }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    console.error('Noryx UI crashed', error, info.componentStack);
  }

  render(): React.ReactNode {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div
        role="alert"
        style={{
          display: 'flex',
          minHeight: '100dvh',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '1.5rem',
          fontFamily: 'system-ui, sans-serif',
          background: '#f7f8fa',
          color: '#10192b',
        }}
      >
        <div style={{ maxWidth: '32rem', textAlign: 'center' }}>
          <h1 style={{ fontSize: '1.125rem', margin: '0 0 0.5rem' }}>
            Une erreur inattendue s’est produite
          </h1>
          <p style={{ fontSize: '0.875rem', lineHeight: 1.6, color: '#56637c', margin: '0 0 1rem' }}>
            L’interface a rencontré un problème. Rechargez la page ; si le problème persiste,
            contactez votre administrateur.
          </p>
          <pre
            style={{
              overflowX: 'auto',
              textAlign: 'left',
              background: '#fff',
              border: '1px solid #dfe3ea',
              borderRadius: '0.5rem',
              padding: '0.75rem',
              fontSize: '0.75rem',
              color: '#8f1a27',
              margin: '0 0 1rem',
            }}
          >
            {error.message}
          </pre>
          <button
            type="button"
            onClick={() => window.location.reload()}
            style={{
              border: 0,
              borderRadius: '0.375rem',
              background: '#0b5fd0',
              color: '#fff',
              padding: '0.5rem 1rem',
              fontSize: '0.875rem',
              fontWeight: 500,
              cursor: 'pointer',
            }}
          >
            Recharger la page
          </button>
        </div>
      </div>
    );
  }
}
