import React from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Footer from './Footer'

export default function MainLayout() {
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900 flex flex-col font-sans">
      <Header />
      <div className="flex-1 w-full flex flex-col items-center">
        <main className="w-full max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8 flex-1 flex flex-col">
          <Outlet />
        </main>
      </div>
      <Footer />
    </div>
  )
}
