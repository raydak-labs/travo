import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './index.css';

async function enableMocking() {
  if (import.meta.env.DEV) {
    // Only load MSW in development
    const { worker } = await import('./mocks/browser');
    return worker.start({ onUnhandledRequest: 'bypass' });
  }
}

// Fire-and-forget on purpose: this bootstrap must NOT block module evaluation, so the
// resulting promise is intentionally left un-awaited. Rejections are still dealt with
// downstream — `.catch` handles MSW startup failures (warn and continue without mocks),
// and the trailing `.then` renders the app once mocking has settled. The leading `void`
// states the fire-and-forget intent explicitly, which is what
// `typescript/no-floating-promises` requires for a deliberately un-awaited promise.
void enableMocking()
  .catch((err) => {
    console.warn('MSW failed to start, continuing without mocks:', err);
  })
  .then(() => {
    createRoot(document.getElementById('root')!).render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
  });
