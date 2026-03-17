import React from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import ProtectedRoute from './components/auth/ProtectedRoute'
import AuthenticatedRoute from './components/auth/AuthenticatedRoute'
import MainLayout from './layouts/MainLayout'
import RequiresDesktop from './components/layout/RequiresDesktop'

// 页面组件
import Login from './pages/Login'
import Register from './pages/Register'
import Problems from './pages/Problems'
import ProblemDetail from './pages/ProblemDetail'
import Contests from './pages/contests'
import ContestDetail from './pages/contestDetail'
import MySubmissions from './pages/MySubmissions'
import Profile from './pages/Profile'
import Scoreboard from './pages/Scoreboard'
import Admin from './pages/Admin'
import MyTeam from './pages/MyTeam'
import TeamProfile from './pages/TeamProfile'

export default function App() {
  return (
    <AuthProvider>
      <Router>
        <Routes>
          <Route path="/" element={<MainLayout />}>
            <Route index element={<RequiresDesktop><Problems /></RequiresDesktop>} />
            <Route path="contests" element={<Contests />} />
            <Route path="contests/:id" element={<ContestDetail />} />
            <Route
              path="login"
              element={
                <AuthenticatedRoute>
                  <Login />
                </AuthenticatedRoute>
              }
            />
            <Route
              path="register"
              element={
                <AuthenticatedRoute>
                  <Register />
                </AuthenticatedRoute>
              }
            />

            {/* 受保护的路由 */}
            <Route
              path="submissions"
              element={
                <ProtectedRoute>
                  <MySubmissions />
                </ProtectedRoute>
              }
            />
            <Route
              path="problem/:id"
              element={
                <ProtectedRoute>
                  <RequiresDesktop>
                    <ProblemDetail />
                  </RequiresDesktop>
                </ProtectedRoute>
              }
            />
            <Route
              path="profile"
              element={
                <ProtectedRoute>
                  <Profile />
                </ProtectedRoute>
              }
            />
            <Route path="profile/:username" element={<Profile />} />
            <Route
              path="team"
              element={
                <ProtectedRoute>
                  <MyTeam />
                </ProtectedRoute>
              }
            />
            <Route path="team/:id" element={<TeamProfile />} />
            <Route
              path="scoreboard"
              element={<Scoreboard />}
            />
            <Route
              path="admin"
              element={
                <ProtectedRoute requiredRole="admin">
                  <RequiresDesktop>
                    <Admin />
                  </RequiresDesktop>
                </ProtectedRoute>
              }
            />

            {/* 404 重定向 */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </Router>
    </AuthProvider>
  )
}
