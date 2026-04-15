import React from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Footer from './Footer'

export default function MainLayout() {
  return (
    <div className="app-shell flex flex-col w-full max-w-full relative">
      <Header />
      <div className="relative flex-1 w-full flex flex-col items-center min-w-0">
        <main className="main-surface glass-baseline flex-1 flex flex-col min-w-0">
          <Outlet />
        </main>
      </div>
      <Footer />
    </div>
  )
}
