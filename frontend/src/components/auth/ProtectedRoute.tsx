import React from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

interface ProtectedRouteProps {
    children: React.ReactNode
    requiredRole?: string
    requiredRoles?: string[]
}

export default function ProtectedRoute({ children, requiredRole, requiredRoles }: ProtectedRouteProps) {
    const { user, isAuthenticated, isLoading } = useAuth()
    const location = useLocation()

    if (isLoading) {
        return <div className="p-10 text-center text-white">正在验证身份...</div>
    }

    if (!isAuthenticated) {
        return <Navigate to="/login" state={{ from: location }} replace />
    }

        const roleChecks = requiredRoles && requiredRoles.length > 0
            ? requiredRoles
            : requiredRole
                ? [requiredRole]
                : []

        if (roleChecks.length > 0 && !roleChecks.includes(user?.role || '')) {
        return <Navigate to="/" replace />
    }

    return <>{children}</>
}
