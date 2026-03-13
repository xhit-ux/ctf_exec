import React from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

interface ProtectedRouteProps {
    children: React.ReactNode
    requiredRole?: 'admin' | 'user'
}

export default function ProtectedRoute({ children, requiredRole }: ProtectedRouteProps) {
    const { user, isAuthenticated, isLoading } = useAuth()
    const location = useLocation()

    if (isLoading) {
        return <div className="p-10 text-center text-white">正在验证身份...</div>
    }

    if (!isAuthenticated) {
        return <Navigate to="/login" state={{ from: location }} replace />
    }

    if (requiredRole && user?.role !== requiredRole) {
        // 如果是普通用户尝试访问 admin 页面，重定向到首页
        return <Navigate to="/" replace />
    }

    return <>{children}</>
}
