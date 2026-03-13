import React, { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import ContestHistory from '../components/profile/ContestHistory'

interface TeamMember {
    id: string
    username: string
    display_name: string
    avatar_url: string
    score: number
    solved_count: number
    role_in_team?: string
}

interface Team {
    id: string
    name: string
    invite_code: string
    leader_id: string
    score: number
    solved_count: number
    members: TeamMember[]
}

export default function MyTeam() {
    const { user, token } = useAuth()
    const [team, setTeam] = useState<Team | null>(null)
    const [loading, setLoading] = useState(true)
    const [createName, setCreateName] = useState('')
    const [inviteCode, setInviteCode] = useState('')
    const [message, setMessage] = useState({ type: '', text: '' })
    const [isProcessing, setIsProcessing] = useState(false)
    const [isEditing, setIsEditing] = useState(false)
    const [editName, setEditName] = useState('')

    const fetchTeam = async () => {
        try {
            const response = await fetch('http://localhost:8080/api/teams/my', {
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            })
            if (response.ok) {
                const data = await response.json()
                setTeam(data)
            } else {
                setTeam(null)
            }
        } catch (err) {
            console.error('Failed to fetch team:', err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        if (token) {
            fetchTeam()
        }
    }, [token])

    const handleCreateTeam = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!createName) return
        setIsProcessing(true)
        setMessage({ type: '', text: '' })

        try {
            const response = await fetch('http://localhost:8080/api/teams', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ name: createName })
            })

            if (response.ok) {
                fetchTeam()
                setMessage({ type: 'success', text: '队伍创建成功！' })
            } else {
                const error = await response.json()
                setMessage({ type: 'error', text: error.error || '创建失败' })
            }
        } catch (err) {
            setMessage({ type: 'error', text: '网络错误' })
        } finally {
            setIsProcessing(false)
        }
    }

    const handleJoinTeam = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!inviteCode) return
        setIsProcessing(true)
        setMessage({ type: '', text: '' })

        try {
            const response = await fetch('http://localhost:8080/api/teams/join', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ invite_code: inviteCode })
            })

            if (response.ok) {
                fetchTeam()
                setMessage({ type: 'success', text: '已加入团队！' })
            } else {
                const error = await response.json()
                setMessage({ type: 'error', text: error.error || '加入失败' })
            }
        } catch (err) {
            setMessage({ type: 'error', text: '网络错误' })
        } finally {
            setIsProcessing(false)
        }
    }

    const handleUpdateName = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!editName || editName === team?.name) {
            setIsEditing(false)
            return
        }
        setIsProcessing(true)
        setMessage({ type: '', text: '' })

        try {
            const response = await fetch('http://localhost:8080/api/teams/update', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ name: editName })
            })

            if (response.ok) {
                setMessage({ type: 'success', text: '团队名称已更新' })
                if (team) setTeam({ ...team, name: editName })
                setIsEditing(false)
                fetchTeam()
            } else {
                const error = await response.json()
                setMessage({ type: 'error', text: error.error || '更新失败' })
            }
        } catch (err) {
            setMessage({ type: 'error', text: '网络错误' })
        } finally {
            setIsProcessing(false)
        }
    }

    const handleKickMember = async (memberId: string, memberName: string) => {
        if (!window.confirm(`确定要将成员 "${memberName}" 移出队伍吗？`)) return
        setIsProcessing(true)

        try {
            const response = await fetch('http://localhost:8080/api/teams/kick', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ user_id: memberId })
            })

            if (response.ok) {
                setMessage({ type: 'success', text: `已移除成员 ${memberName}` })
                fetchTeam()
            } else {
                const error = await response.json()
                alert(error.error || '移除失败')
            }
        } catch (err) {
            alert('网络错误')
        } finally {
            setIsProcessing(false)
        }
    }

    const handleLeaveTeam = async () => {
        if (!window.confirm('确定要退出队伍吗？')) return
        setIsProcessing(true)

        try {
            const response = await fetch('http://localhost:8080/api/teams/leave', {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            })

            if (response.ok) {
                setTeam(null)
                setMessage({ type: 'success', text: '已退出队伍' })
            } else {
                const error = await response.json()
                alert(error.error || '退出失败')
            }
        } catch (err) {
            alert('网络错误')
        } finally {
            setIsProcessing(false)
        }
    }

    const handleDissolveTeam = async () => {
        if (!window.confirm('确定要解散队伍吗？这将注销所有成员的队伍状态。')) return
        setIsProcessing(true)

        try {
            const response = await fetch('http://localhost:8080/api/teams/dissolve', {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            })

            if (response.ok) {
                setTeam(null)
                setMessage({ type: 'success', text: '队伍已解散' })
            } else {
                const error = await response.json()
                alert(error.error || '解散失败')
            }
        } catch (err) {
            alert('网络错误')
        } finally {
            setIsProcessing(false)
        }
    }

    if (loading) return <div className="p-8 text-center text-gray-500">正在获取队伍信息...</div>

    return (
        <div className="max-w-6xl mx-auto space-y-8 p-4 lg:p-8">
            <div>
                <h1 className="text-3xl font-extrabold text-gray-900 tracking-tight">我的队伍</h1>
                <p className="text-gray-500 font-medium mt-2">
                    在这里管理你的 CTF 团队，邀请成员共同征战。
                </p>
            </div>

            {message.text && (
                <div className={`p-4 rounded-xl flex items-center gap-3 shadow-sm ${message.type === 'success' ? 'bg-green-50 text-green-700 border border-green-100' : 'bg-red-50 text-red-700 border border-red-100'
                    }`}>
                    <span>{message.text}</span>
                </div>
            )}

            {!team ? (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                    {/* 创建队伍 */}
                    <div className="bg-white border rounded-xl p-8 shadow-sm">
                        <h2 className="text-xl font-semibold mb-4 text-gray-900">创建新队伍</h2>
                        <form onSubmit={handleCreateTeam} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1">团队名称</label>
                                <input
                                    type="text"
                                    value={createName}
                                    onChange={(e) => setCreateName(e.target.value)}
                                    className="w-full px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                                    placeholder="给你的队伍起个响亮的名字"
                                />
                            </div>
                            <button
                                type="submit"
                                disabled={isProcessing || !createName}
                                className="w-full bg-blue-600 text-white py-2 rounded-lg font-medium hover:bg-blue-700 transition disabled:opacity-50"
                            >
                                {isProcessing ? '处理中...' : '立即创建'}
                            </button>
                        </form>
                    </div>

                    {/* 加入队伍 */}
                    <div className="bg-white border rounded-xl p-8 shadow-sm">
                        <h2 className="text-xl font-semibold mb-4 text-gray-900">加入已有队伍</h2>
                        <form onSubmit={handleJoinTeam} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1">邀请码</label>
                                <input
                                    type="text"
                                    value={inviteCode}
                                    onChange={(e) => setInviteCode(e.target.value.toUpperCase())}
                                    className="w-full px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                                    placeholder="输入 8 位邀请码"
                                />
                            </div>
                            <button
                                type="submit"
                                disabled={isProcessing || !inviteCode}
                                className="w-full bg-gray-800 text-white py-2 rounded-lg font-medium hover:bg-gray-900 transition disabled:opacity-50"
                            >
                                {isProcessing ? '处理中...' : '加入队伍'}
                            </button>
                        </form>
                    </div>
                </div>
            ) : (
                <div className="bg-white border rounded-xl overflow-hidden shadow-sm">
                    <div className="p-8 bg-gradient-to-r from-pink-300/70 to-rose-200/70 text-rose-950 relative">
                        <div className="flex justify-between items-start">
                            <div>
                                <h2 className="text-3xl font-black mb-1">{team.name}</h2>
                                <div className="flex items-center gap-4 mt-2">
                                    <div className="bg-white/40 border border-white/50 px-3 py-1 rounded-full text-xs font-bold uppercase backdrop-blur-sm">
                                        邀请码: <span className="text-rose-600 font-mono tracking-widest ml-1">{team.invite_code}</span>
                                    </div>
                                    <div className="text-sm font-medium opacity-80 text-rose-900">
                                        成员: {team.members?.length || 0} / 5
                                    </div>
                                </div>
                            </div>
                            <div className="text-right">
                                <p className="text-3xl font-black">{team.score}</p>
                                <p className="text-xs font-bold uppercase opacity-60">团队总积分</p>
                            </div>
                        </div>
                    </div>

                    <div className="p-8">
                        <div className="flex justify-between items-center mb-6">
                            <h3 className="text-sm font-bold text-gray-400 uppercase tracking-wider">团队成员</h3>
                            {team.leader_id === user?.id && !isEditing && (
                                <button
                                    onClick={() => {
                                        setEditName(team.name)
                                        setIsEditing(true)
                                    }}
                                    className="text-xs font-bold text-blue-600 hover:text-blue-700 flex items-center gap-1"
                                >
                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                    </svg>
                                    修改队名
                                </button>
                            )}
                        </div>

                        {isEditing && (
                            <form onSubmit={handleUpdateName} className="mb-8 p-6 bg-blue-50 rounded-xl border border-blue-100 animate-in fade-in slide-in-from-top-2">
                                <label className="block text-sm font-bold text-blue-700 mb-2">更新团队名称</label>
                                <div className="flex gap-2">
                                    <input
                                        type="text"
                                        value={editName}
                                        onChange={(e) => setEditName(e.target.value)}
                                        className="flex-1 px-4 py-2 border border-blue-200 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none"
                                        placeholder="输入新的队名"
                                        autoFocus
                                    />
                                    <button
                                        type="submit"
                                        disabled={isProcessing || !editName || editName === team.name}
                                        className="bg-blue-600 text-white px-6 py-2 rounded-lg font-bold hover:bg-blue-700 transition disabled:opacity-50"
                                    >
                                        保存
                                    </button>
                                    <button
                                        type="button"
                                        onClick={() => setIsEditing(false)}
                                        className="px-4 py-2 text-gray-500 font-bold hover:text-gray-700"
                                    >
                                        取消
                                    </button>
                                </div>
                            </form>
                        )}

                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                            {team.members?.map((member) => (
                                <div key={member.id} className="flex items-center gap-4 p-4 bg-gray-50 rounded-xl border border-gray-100 hover:border-blue-100 transition-colors group">
                                    <div className="relative">
                                        <div className="w-12 h-12 rounded-full overflow-hidden bg-white border border-gray-200">
                                            {member.avatar_url ? (
                                                <img src={member.avatar_url.startsWith('static/') ? `http://localhost:8080/${member.avatar_url}` : member.avatar_url} alt={member.username} className="w-full h-full object-cover" />
                                            ) : (
                                                <div className="w-full h-full flex items-center justify-center font-bold text-blue-500 bg-blue-50">
                                                    {member.username[0].toUpperCase()}
                                                </div>
                                            )}
                                        </div>
                                        {member.id === team.leader_id && (
                                            <div className="absolute -top-1 -right-1 w-4 h-4 bg-yellow-400 text-yellow-900 rounded-full flex items-center justify-center border border-white" title="队长">
                                                <svg className="w-2.5 h-2.5" fill="currentColor" viewBox="0 0 20 20">
                                                    <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                                                </svg>
                                            </div>
                                        )}
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <div className="flex items-center gap-2">
                                            <p className="font-bold text-gray-900 truncate">{member.display_name || member.username}</p>
                                            {member.role_in_team === 'leader' && (
                                                <span className="px-1.5 py-0.5 bg-yellow-100 text-yellow-700 text-[10px] font-black uppercase rounded">队长</span>
                                            )}
                                        </div>
                                        <p className="text-xs text-gray-500 font-medium">@{member.username}</p>
                                    </div>
                                    <div className="text-right flex items-center gap-3">
                                        <div className="flex flex-col items-end">
                                            <p className="text-sm font-bold text-blue-600">{member.score}</p>
                                            {team.leader_id === user?.id && member.id !== user?.id && (
                                                <button
                                                    onClick={() => handleKickMember(member.id, member.display_name || member.username)}
                                                    disabled={isProcessing}
                                                    className="text-[10px] font-black text-red-400 hover:text-red-600 uppercase tracking-tighter mt-1"
                                                >
                                                    移出
                                                </button>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>

                        <div className="mt-8">
                             <ContestHistory />
                        </div>

                        <div className="mt-12 flex justify-between items-center border-t pt-8">
                            <p className="text-sm text-gray-400 italic">只有队长可以解散队伍，普通成员可以直接退出。</p>
                            {team.leader_id === user?.id ? (
                                <button
                                    onClick={handleDissolveTeam}
                                    disabled={isProcessing}
                                    className="px-6 py-2 bg-red-50 text-red-600 rounded-lg font-bold hover:bg-red-100 transition border border-red-100"
                                >
                                    解散队伍
                                </button>
                            ) : (
                                <button
                                    onClick={handleLeaveTeam}
                                    disabled={isProcessing}
                                    className="px-6 py-2 bg-orange-50 text-orange-600 rounded-lg font-bold hover:bg-orange-100 transition border border-orange-100"
                                >
                                    退出队伍
                                </button>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
