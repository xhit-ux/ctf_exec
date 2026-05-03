import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './index.css'
import AppErrorBoundary from './components/feedback/AppErrorBoundary'
import GlobalErrorToaster from './components/feedback/GlobalErrorToaster'
import { ThemeProvider } from './context/ThemeContext'

const container = document.getElementById('root')!
const root = createRoot(container)
root.render(
  <React.StrictMode>
    <ThemeProvider>
      <AppErrorBoundary>
        <GlobalErrorToaster>
          <App />
        </GlobalErrorToaster>
      </AppErrorBoundary>
    </ThemeProvider>
  </React.StrictMode>
)
