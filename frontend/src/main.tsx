import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './index.css'
import AppErrorBoundary from './components/feedback/AppErrorBoundary'
import GlobalErrorToaster from './components/feedback/GlobalErrorToaster'

const container = document.getElementById('root')!
const root = createRoot(container)
root.render(
  <React.StrictMode>
    <AppErrorBoundary>
      <GlobalErrorToaster>
        <App />
      </GlobalErrorToaster>
    </AppErrorBoundary>
  </React.StrictMode>
)
