import React from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Sidebar from './Sidebar'
import Footer from './Footer'

export default function Layout() {
  return (
    <div className="min-h-screen bg-gray-100 text-gray-900 flex flex-col">
      <Header />
      <div className="max-w-7xl mx-auto py-6 px-4 flex-1 w-full">
        <div className="relative">
          {/* Floating Sidebar */}
          <aside className="hidden lg:block fixed left-10 top-24 w-60 z-40">
            <div className="bg-white rounded-lg p-4 shadow-lg border border-gray-200">
              <Sidebar />
            </div>
          </aside>
          
          {/* Main Content - offset for fixed sidebar */}
          <main className="lg:ml-40 lg:pr-10">
            <div className="bg-white rounded-lg p-6 shadow-sm min-h-[60vh]">
              <Outlet />
            </div>
          </main>
        </div>
      </div>
      <Footer />
    </div>
  )
}
