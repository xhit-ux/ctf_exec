import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

interface LeaderboardUser {
    rank: number
    username: string
    display_name: string
    score: number
    solved_count: number
}

interface LeaderboardTeam {
    rank: number
    id: string
    name: string
    total_score: number
    avg_score: number
    member_count: number
}

export default function Scoreboard() {
    const [userRankings, setUserRankings] = useState<LeaderboardUser[]>([])
    const [teamRankings, setTeamRankings] = useState<LeaderboardTeam[]>([])
    const [activeTab, setActiveTab] = useState<'user' | 'team'>('user')
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')

    const fetchData = async () => {
        setLoading(true)
        setError('')
        try {
            const endpoint = activeTab === 'user'
                ? 'http://localhost:8080/api/users/leaderboard'
                : 'http://localhost:8080/api/teams/leaderboard'

            const res = await fetch(endpoint)
            if (!res.ok) {
                throw new Error('获取排行榜失败')
            }
            const data = await res.json()
            if (activeTab === 'user') {
                setUserRankings(data)
            } else {
                setTeamRankings(data)
            }
        } catch (err) {
            setError('无法加载排行榜')
            console.error(err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchData()
    }, [activeTab])

    return (
        <div className="space-y-6">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b pb-4">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">排行榜</h1>
                    <p className="text-gray-500 text-sm">看看谁才是最强练习者</p>
                </div>

                <div className="flex bg-gray-100 p-1 rounded-lg">
                    <button
                        onClick={() => setActiveTab('user')}
                        className={`px-4 py-1.5 rounded-md text-sm font-bold transition ${activeTab === 'user' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'
                            }`}
                    >
                        个人排行
                    </button>
                    <button
                        onClick={() => setActiveTab('team')}
                        className={`px-4 py-1.5 rounded-md text-sm font-bold transition ${activeTab === 'team' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'
                            }`}
                    >
                        团队排行
                    </button>
                </div>
            </div>

            {error && <div className="p-4 bg-red-50 text-red-600 rounded-lg">{error}</div>}

            <div className="bg-white border rounded-xl overflow-hidden shadow-sm min-h-[400px] flex flex-col">
                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr className="bg-gray-50 text-gray-400 text-xs font-black uppercase tracking-widest border-b">
                            <th className="px-6 py-4 w-20">排名</th>
                            <th className="px-6 py-4">{activeTab === 'user' ? '练习者' : '团队名称'}</th>
                            <th className="px-6 py-4 text-center">{activeTab === 'user' ? '积分' : '人均积分'}</th>
                            <th className="px-6 py-4 text-center">{activeTab === 'user' ? '解题数' : '总积分'}</th>
                            {activeTab === 'team' && <th className="px-6 py-4 text-center">成员数</th>}
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                        {activeTab === 'user' ? (
                            userRankings.map((entry) => (
                                <tr key={entry.rank} className="hover:bg-gray-50 transition-colors">
                                    <td className="px-6 py-4">
                                        <span className={`flex items-center justify-center w-8 h-8 rounded-lg font-black ${entry.rank === 1 ? 'bg-yellow-100 text-yellow-700' :
                                            entry.rank === 2 ? 'bg-gray-100 text-gray-600' :
                                                entry.rank === 3 ? 'bg-orange-100 text-orange-700' :
                                                    'text-gray-400'
                                            }`}>
                                            {entry.rank}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4">
                                        <Link to={`/profile/${entry.username}`} className="font-bold text-gray-900 hover:text-blue-600 transition">
                                            {entry.display_name || entry.username}
                                        </Link>
                                        <p className="text-xs text-gray-400 font-medium">@{entry.username}</p>
                                    </td>
                                    <td className="px-6 py-4 text-center font-black text-blue-600">{entry.score}</td>
                                    <td className="px-6 py-4 text-center font-bold text-gray-500">{entry.solved_count}</td>
                                </tr>
                            ))
                        ) : (
                            teamRankings.map((entry) => (
                                <tr key={entry.rank} className="hover:bg-gray-50 transition-colors">
                                    <td className="px-6 py-4">
                                        <span className={`flex items-center justify-center w-8 h-8 rounded-lg font-black ${entry.rank === 1 ? 'bg-yellow-100 text-yellow-700' :
                                            entry.rank === 2 ? 'bg-gray-100 text-gray-600' :
                                                entry.rank === 3 ? 'bg-orange-100 text-orange-700' :
                                                    'text-gray-400'
                                            }`}>
                                            {entry.rank}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4">
                                        <Link to={`/team/${entry.id}`} className="font-bold text-gray-900 hover:text-blue-600 transition">
                                            {entry.name}
                                        </Link>
                                    </td>
                                    <td className="px-6 py-4 text-center font-black text-blue-600">
                                        {entry.avg_score.toFixed(1)}
                                    </td>
                                    <td className="px-6 py-4 text-center font-bold text-gray-500">
                                        {entry.total_score}
                                    </td>
                                    <td className="px-6 py-4 text-center font-bold text-gray-400">
                                        {entry.member_count}
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>

                {loading && (
                    <div className="flex-1 flex items-center justify-center p-12">
                        <div className="flex flex-col items-center gap-3">
                            <div className="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
                            <p className="text-gray-400 text-sm font-medium">正在加载排名列表...</p>
                        </div>
                    </div>
                )}

                {!loading && (
                    (activeTab === 'user' && userRankings.length === 0) ||
                    (activeTab === 'team' && teamRankings.length === 0)
                ) && (
                        <div className="flex-1 flex items-center justify-center p-12 text-center text-gray-400 font-medium">
                            <div className="max-w-xs">
                                <p className="font-bold text-lg mb-1">虚位以待</p>
                                <p className="text-sm italic">暂无数据记录，期待您榜上有名！</p>
                            </div>
                        </div>
                    )}
            </div>
        </div>
    )
}
