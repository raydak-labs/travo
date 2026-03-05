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

enableMocking()
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
