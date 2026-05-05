import React from 'react'

export default function Footer() {
  return (
    <footer className="mt-6 border-t border-amber-100/80 bg-white/80 backdrop-blur-md dark:border-gray-700/80 dark:bg-gray-900/75">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-5 text-sm text-gray-500 dark:text-gray-400 flex flex-col sm:flex-row items-center justify-between gap-2">
        <span>© CTF Platform</span>
        <span className="text-xs tracking-wide uppercase text-gray-400 dark:text-gray-500">Practice · Contest · Team</span>
      </div>
    </footer>
  )
}
