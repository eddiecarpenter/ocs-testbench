import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router';

import { App } from './App';
import { appConfig } from './config/app';

import '@mantine/core/styles.css';
import '@mantine/charts/styles.css';
import '@mantine/notifications/styles.css';

async function bootstrap() {
  if (appConfig.useMockApi) {
    // Dynamic import keeps axios-mock-adapter and the fixtures out of the
    // production bundle when `VITE_USE_MOCK_API` is false at build time.
    await import('./mocks');
    if (appConfig.debugApi) {
      console.info('[api] mock layer enabled');
    }
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </StrictMode>,
  );
}

void bootstrap();
