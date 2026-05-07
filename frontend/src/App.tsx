import React from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate, useParams } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import ProtectedRoute from './components/auth/ProtectedRoute'
import AuthenticatedRoute from './components/auth/AuthenticatedRoute'
import MainLayout from './layouts/MainLayout'
import RequiresDesktop from './components/layout/RequiresDesktop'

// 页面组件
import Login from './pages/Login'
import Register from './pages/Register'
import ForgotPassword from './pages/ForgotPassword'
import Index from './pages/Index'
import Problems from './pages/Problems'
import ProblemDetail from './pages/ProblemDetail'
import ProblemPublish from './pages/ProblemPublish'
import Contests from './pages/contests'
import MySubmissions from './pages/MySubmissions'
import Profile from './pages/Profile'
import Scoreboard from './pages/Scoreboard'
import Admin from './pages/Admin'
import MyTeam from './pages/MyTeam'
import TeamProfile from './pages/TeamProfile'

import EditProfile from './pages/EditProfile'
import FollowList from './pages/FollowList'
import ContestLayout from './pages/contest/ContestLayout'
import ContestOverview from './pages/contest/ContestOverview'
import ContestProblems from './pages/contest/ContestProblems'
import ContestScoreboard from './pages/contest/ContestScoreboard'
import ContestMySubmissions from './pages/contest/ContestMySubmissions'
import ContestParticipants from './pages/contest/ContestParticipants'
import ContestAdmin from './pages/contest/ContestAdmin'
import ContestListAdminPage from './pages/admin/ContestList'
import ContestEditAdminPage from './pages/admin/ContestEdit'

function LegacyContestRedirect() {
  const { id } = useParams()
  return <Navigate to={id ? `/contest/${id}` : '/contests'} replace />
}

export default function App() {
  return (
    <AuthProvider>
      <Router>
        <Routes>
          <Route path="/" element={<MainLayout />}>
            <Route index element={<Index />} />
            <Route
              path="problem"
              element={
                <ProtectedRoute>
                  <RequiresDesktop>
                    <Problems />
                  </RequiresDesktop>
                </ProtectedRoute>
              }
            />
            <Route path="contests" element={<Contests />} />
            <Route path="contests/:id" element={<LegacyContestRedirect />} />
            <Route path="contest/:contestId" element={<ContestLayout />}>
              <Route index element={<ContestOverview />} />
              <Route path="problems" element={<ContestProblems />} />
              <Route path="board" element={<ContestScoreboard />} />
              <Route
                path="submissions"
                element={
                  <ProtectedRoute>
                    <ContestMySubmissions />
                  </ProtectedRoute>
                }
              />
              <Route
                path="participants"
                element={
                  <ProtectedRoute>
                    <ContestParticipants />
                  </ProtectedRoute>
                }
              />
              <Route
                path="admin"
                element={
                  <ProtectedRoute>
                    <ContestAdmin />
                  </ProtectedRoute>
                }
              />
            </Route>
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
            <Route
              path="forgot-password"
              element={
                <AuthenticatedRoute>
                  <ForgotPassword />
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
              path="problem/publish"
              element={
                <ProtectedRoute requiredRoles={["root", "site_admin", "contest_admin"]}>
                  <RequiresDesktop>
                    <ProblemPublish />
                  </RequiresDesktop>
                </ProtectedRoute>
              }
            />
            <Route
              path="problem/:id/edit"
              element={
                <ProtectedRoute>
                  <RequiresDesktop>
                    <ProblemPublish />
                  </RequiresDesktop>
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
              path="profile/:username/edit" 
              element={
                <ProtectedRoute>
                  <EditProfile />
                </ProtectedRoute>
              } 
            />
            <Route path="profile/:username/followers" element={<FollowList type="followers" />} />
            <Route path="profile/:username/following" element={<FollowList type="following" />} />
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
                <ProtectedRoute requiredRoles={["root", "site_admin"]}>
                  <RequiresDesktop>
                    <Admin />
                  </RequiresDesktop>
                </ProtectedRoute>
              }
            />
            <Route
              path="admin/contests"
              element={
                <ProtectedRoute requiredRoles={["root", "site_admin"]}>
                  <RequiresDesktop>
                    <ContestListAdminPage />
                  </RequiresDesktop>
                </ProtectedRoute>
              }
            />
            <Route
              path="admin/contests/:contestId/edit"
              element={
                <ProtectedRoute requiredRoles={["root", "site_admin"]}>
                  <RequiresDesktop>
                    <ContestEditAdminPage />
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
