import React, { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

interface TeamMember {
    id: string
    username: string
    display_name: string
    avatar_url: string
    score: number
    solved_count: number
    role_in_team: string
}

interface Team {
    id: string
    name: string
    invite_code?: string
    leader_id: string
    score: number
    solved_count: number
    members: TeamMember[]
}

export default function TeamProfile() {
    const { id: teamId } = useParams<{ id: string }>()
    const { user, token } = useAuth()
    const navigate = useNavigate()

    const [team, setTeam] = useState<Team | null>(null)
    const [loading, setLoading] = useState(true)
    const [isEditing, setIsEditing] = useState(false)
    const [editName, setEditName] = useState('')
    const [isProcessing, setIsProcessing] = useState(false)
    const [message, setMessage] = useState({ type: '', text: '' })

    const fetchTeam = async () => {
        setLoading(true)
        try {
            const response = await fetch(`http://localhost:8080/api/teams/public?id=${teamId}`, {
                headers: token ? { 'Authorization': `Bearer ${token}` } : {}
            })
            if (response.ok) {
                const data = await response.json()
                setTeam(data)
                setEditName(data.name)
            } else {
                setTeam(null)
            }
        } catch (err) {
            console.error('Failed to fetch team profile:', err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchTeam()
    }, [teamId, token])

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
                fetchTeam() // 重新获取成员列表
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

    if (loading) return <div className="p-10 text-center text-gray-500 italic">正在加载团队资料...</div>

    if (!team) {
        return (
            <div className="p-10 text-center text-gray-500">
                <p className="mb-4">未找到该团队信息</p>
                <Link to="/scoreboard" className="text-blue-600 hover:underline">返回排行榜</Link>
            </div>
        )
    }

    const isLeader = user?.id === team.leader_id

    return (
        <div className="max-w-5xl mx-auto space-y-8">
            <div className="bg-white border rounded-2xl overflow-hidden shadow-sm">
                <div className="h-48 bg-gradient-to-br from-indigo-600 via-blue-700 to-cyan-500 p-8 relative">
                    <div className="flex justify-between items-start text-white">
                        <div className="space-y-2">
                            {isEditing ? (
                                <form onSubmit={handleUpdateName} className="flex items-center gap-2">
                                    <input
                                        type="text"
                                        value={editName}
                                        onChange={(e) => setEditName(e.target.value)}
                                        className="text-3xl font-black bg-white/10 border border-white/20 rounded px-2 py-1 outline-none focus:bg-white/20 transition text-white placeholder-white/50"
                                        autoFocus
                                    />
                                    <button
                                        type="submit"
                                        disabled={isProcessing}
                                        className="bg-white text-blue-700 px-3 py-1.5 rounded-lg text-sm font-bold shadow-sm hover:bg-gray-100 transition"
                                    >
                                        保存
                                    </button>
                                    <button
                                        type="button"
                                        onClick={() => setIsEditing(false)}
                                        className="text-white/80 hover:text-white text-sm font-bold px-2"
                                    >
                                        取消
                                    </button>
                                </form>
                            ) : (
                                <div className="flex items-center gap-3">
                                    <h1 className="text-4xl font-black tracking-tight">{team.name}</h1>
                                    {isLeader && (
                                        <button
                                            onClick={() => setIsEditing(true)}
                                            className="px-2 py-1 bg-white/20 hover:bg-white/30 rounded text-xs font-bold transition flex items-center gap-1"
                                        >
                                            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                            </svg>
                                            修改名称
                                        </button>
                                    )}
                                </div>
                            )}
                            <div className="flex items-center gap-4 text-white/80">
                                {team.invite_code && (
                                    <div className="bg-white/10 border border-white/20 px-3 py-1 rounded-full text-xs font-bold flex items-center gap-2 backdrop-blur-sm">
                                        邀请码: <span className="text-yellow-300 font-mono tracking-widest">{team.invite_code}</span>
                                    </div>
                                )}
                                <div className="text-sm font-bold">
                                    成员数: {team.members.length} / 5
                                </div>
                            </div>
                        </div>

                        <div className="text-right">
                            <div className="bg-black/10 backdrop-blur-md rounded-2xl p-4 border border-white/10 inline-block">
                                <p className="text-4xl font-black leading-none mb-1">{team.score}</p>
                                <p className="text-[10px] font-black uppercase tracking-widest opacity-60">团队总积分</p>
                            </div>
                        </div>
                    </div>
                </div>

                <div className="p-8">
                    {message.text && (
                        <div className={`mb-8 p-4 rounded-xl flex items-center gap-3 border ${message.type === 'success' ? 'bg-green-50 text-green-700 border-green-100' : 'bg-red-50 text-red-700 border-red-100'
                            }`}>
                            <span>{message.text}</span>
                        </div>
                    )}

                    <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
                        <div className="md:col-span-2">
                            <h3 className="text-xs font-black text-gray-400 uppercase tracking-widest mb-6">团队成员</h3>
                            <div className="grid grid-cols-1 gap-4">
                                {team.members.map((member) => (
                                    <div key={member.id} className="flex items-center gap-4 p-5 bg-gray-50 rounded-2xl border border-gray-100 hover:border-blue-200 transition-colors group">
                                        <div className="relative">
                                            <div className="w-14 h-14 rounded-2xl overflow-hidden bg-white border border-gray-200 shadow-sm">
                                                {member.avatar_url ? (
                                                    <img src={member.avatar_url.startsWith('static/') ? `http://localhost:8080/${member.avatar_url}` : member.avatar_url} alt={member.username} className="w-full h-full object-cover" />
                                                ) : (
                                                    <div className="w-full h-full flex items-center justify-center font-bold text-blue-500 bg-blue-50 text-xl">
                                                        {member.username[0].toUpperCase()}
                                                    </div>
                                                )}
                                            </div>
                                            {member.id === team.leader_id && (
                                                <div className="absolute -top-1 -right-1 w-5 h-5 bg-yellow-400 text-yellow-900 rounded-full flex items-center justify-center border-2 border-white" title="队长">
                                                    <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                                                        <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                                                    </svg>
                                                </div>
                                            )}
                                        </div>
                                        <div className="flex-1 min-w-0">
                                            <div className="flex items-center gap-2">
                                                <Link to={`/profile/${member.username}`} className="font-bold text-gray-900 truncate hover:text-blue-600 transition">
                                                    {member.display_name || member.username}
                                                </Link>
                                                {member.role_in_team === 'leader' && (
                                                    <span className="px-1.5 py-0.5 bg-yellow-100 text-yellow-700 text-[10px] font-black uppercase rounded">队长</span>
                                                )}
                                            </div>
                                            <p className="text-xs text-gray-500 font-medium">@{member.username}</p>
                                        </div>
                                        <div className="text-right flex items-center gap-6">
                                            <div>
                                                <p className="text-lg font-black text-blue-600">{member.score}</p>
                                                <p className="text-[10px] font-bold text-gray-400 uppercase">贡献积分</p>
                                            </div>

                                            {isLeader && member.id !== user?.id && (
                                                <button
                                                    onClick={() => handleKickMember(member.id, member.display_name || member.username)}
                                                    disabled={isProcessing}
                                                    className="flex items-center gap-1 px-2 py-1 bg-red-50 text-red-500 hover:bg-red-100 rounded text-[10px] font-black uppercase transition-all"
                                                    title="移出队伍"
                                                >
                                                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 7a4 4 0 11-8 0 4 4 0 018 0zM9 14a6 6 0 00-6 6v1h12v-1a6 6 0 00-6-6zM21 12h-6" />
                                                    </svg>
                                                    踢出
                                                </button>
                                            )}
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>

                        <div className="space-y-6">
                            <div className="bg-gray-50 border border-gray-100 rounded-2xl p-6">
                                <h3 className="text-xs font-black text-gray-400 uppercase tracking-widest mb-4">解题战报</h3>
                                <div className="space-y-4">
                                    <div className="flex justify-between items-center bg-white p-3 rounded-xl border border-gray-100">
                                        <span className="text-sm font-medium text-gray-500">累计解题</span>
                                        <span className="text-lg font-black text-gray-900">{team.solved_count}</span>
                                    </div>
                                    <div className="flex justify-between items-center bg-white p-3 rounded-xl border border-gray-100">
                                        <span className="text-sm font-medium text-gray-500">人均贡献</span>
                                        <span className="text-lg font-black text-blue-600">
                                            {(team.score / (team.members.length || 1)).toFixed(1)}
                                        </span>
                                    </div>
                                </div>
                            </div>

                            <div className="flex items-center gap-3 p-4 bg-blue-50/50 rounded-2xl border border-blue-100/50">
                                <div className="p-2 bg-blue-100 text-blue-600 rounded-xl">
                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                    </svg>
                                </div>
                                <div>
                                    <p className="text-[10px] text-gray-400 font-black uppercase">创建于</p>
                                    <p className="text-sm font-bold text-gray-900">
                                        {new Date(team.members[0]?.id === team.leader_id ? team.members[0].id : team.leader_id).toLocaleDateString() === 'Invalid Date'
                                            ? '未知'
                                            : new Date().toLocaleDateString() // 这里后端没返回队伍创建时间，暂时用占位符
                                        }
                                    </p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div className="flex justify-between items-center text-gray-400 text-sm italic px-4">
                <p>只有队长可以修改队伍名称或移除队员。邀请码仅对本队成员可见。</p>
                <Link to="/scoreboard" className="font-bold hover:text-blue-600 transition">← 返回排行榜</Link>
            </div>
        </div>
    )
}
