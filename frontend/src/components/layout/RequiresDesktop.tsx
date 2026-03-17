import React, { useState, useEffect } from 'react';

interface RequiresDesktopProps {
  children: React.ReactNode;
}

export default function RequiresDesktop({ children }: RequiresDesktopProps) {
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const checkScreenSize = () => {
      setIsMobile(window.innerWidth < 768);
    };

    checkScreenSize();
    window.addEventListener('resize', checkScreenSize);

    return () => window.removeEventListener('resize', checkScreenSize);
  }, []);

  if (isMobile) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] text-center px-4">
        <svg className="w-16 h-16 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
        <h2 className="text-xl font-bold text-gray-900 mb-2">设备宽度不足</h2>
        <p className="text-gray-500 max-w-md">
          当前页面功能复杂以至于无法在移动设备上正常显示，请使用桌面端浏览器进行查看和操作。
        </p>
      </div>
    );
  }

  return <>{children}</>;
}
