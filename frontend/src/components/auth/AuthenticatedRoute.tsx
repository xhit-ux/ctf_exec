import { Navigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

interface AuthenticatedRouteProps {
    children: React.ReactNode
}

// 已登录用户访问登录/注册页面时重定向到首页
export default function AuthenticatedRoute({ children }: AuthenticatedRouteProps) {
    const { isAuthenticated, isLoading } = useAuth()

    if (isLoading) {
        return <div className="p-10 text-center text-white">正在验证身份...</div>
    }

    if (isAuthenticated) {
        return <Navigate to="/" replace />
    }

    return <>{children}</>
}
