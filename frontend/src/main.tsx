import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './app/App';
import { applyBrandOverrides } from './lib/config';
import './styles/globals.css';

applyBrandOverrides();

const container = document.getElementById('root');
if (!container) throw new Error('Missing #root container');

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
