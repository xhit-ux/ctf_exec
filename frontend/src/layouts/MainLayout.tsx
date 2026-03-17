import React from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Footer from './Footer'

export default function MainLayout() {
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900 flex flex-col font-sans w-full max-w-full overflow-x-hidden">
      <Header />
      <div className="flex-1 w-full flex flex-col items-center min-w-0">
        <main className="w-full max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8 flex-1 flex flex-col min-w-0">
          <Outlet />
        </main>
      </div>
      <Footer />
    </div>
  )
}
