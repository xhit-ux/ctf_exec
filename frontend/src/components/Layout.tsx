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
        <div className="grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-6">
          <aside className="hidden lg:block">
            <div className="sticky top-6">
              <Sidebar />
            </div>
          </aside>
          <main>
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
