import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiUrl } from '../utils/api'
import Pagination from '../components/common/Pagination'
import { fromUnknownError, readSafeErrorMessage } from '../utils/error'

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

interface PaginatedResponse<T> {
    items: T[]
    page: number
    page_size: number
    total: number
    total_pages: number
}

export default function Scoreboard() {
    const [userRankings, setUserRankings] = useState<LeaderboardUser[]>([])
    const [teamRankings, setTeamRankings] = useState<LeaderboardTeam[]>([])
    const [activeTab, setActiveTab] = useState<'user' | 'team'>('user')
    const [userPage, setUserPage] = useState(1)
    const [teamPage, setTeamPage] = useState(1)
    const [userPageSize, setUserPageSize] = useState(5)
    const [teamPageSize, setTeamPageSize] = useState(5)
    const [userTotal, setUserTotal] = useState(0)
    const [teamTotal, setTeamTotal] = useState(0)
    const [userTotalPages, setUserTotalPages] = useState(0)
    const [teamTotalPages, setTeamTotalPages] = useState(0)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [searchInput, setSearchInput] = useState('')
    const [searchLoading, setSearchLoading] = useState(false)
    const [searchError, setSearchError] = useState('')
    const [pinnedUser, setPinnedUser] = useState<LeaderboardUser | null>(null)

    const currentPage = activeTab === 'user' ? userPage : teamPage
    const currentPageSize = activeTab === 'user' ? userPageSize : teamPageSize
    const currentTotalPages = activeTab === 'user' ? userTotalPages : teamTotalPages
    const currentTotal = activeTab === 'user' ? userTotal : teamTotal

    const fetchData = async () => {
        setLoading(true)
        setError('')
        try {
            const endpoint = activeTab === 'user'
                ? apiUrl(`/api/users/leaderboard?page=${userPage}&page_size=${userPageSize}`)
                : apiUrl(`/api/teams/leaderboard?page=${teamPage}&page_size=${teamPageSize}`)

            const res = await fetch(endpoint)
            if (!res.ok) {
                throw new Error('获取排行榜失败')
            }
            const data = await res.json() as PaginatedResponse<LeaderboardUser> | PaginatedResponse<LeaderboardTeam>
            if (activeTab === 'user') {
                const userData = data as PaginatedResponse<LeaderboardUser>
                setUserRankings(userData.items)
                setUserTotal(userData.total)
                setUserTotalPages(userData.total_pages)
            } else {
                const teamData = data as PaginatedResponse<LeaderboardTeam>
                setTeamRankings(teamData.items)
                setTeamTotal(teamData.total)
                setTeamTotalPages(teamData.total_pages)
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
    }, [activeTab, userPage, teamPage, userPageSize, teamPageSize])

    const handleSearchUser = async () => {
        const username = searchInput.trim()
        if (!username) {
            setSearchError('请输入用户名')
            return
        }

        setSearchLoading(true)
        setSearchError('')
        try {
            const res = await fetch(apiUrl(`/api/users/leaderboard/search?username=${encodeURIComponent(username)}`))
            if (!res.ok) {
                setPinnedUser(null)
                setSearchError(await readSafeErrorMessage(res, '检索失败，请稍后重试'))
                return
            }

            const data = await res.json()

            setPinnedUser(data as LeaderboardUser)
        } catch (err) {
            setPinnedUser(null)
            setSearchError(fromUnknownError(err, '检索失败，请稍后重试'))
        } finally {
            setSearchLoading(false)
        }
    }

    const clearPinnedUser = () => {
        setPinnedUser(null)
        setSearchError('')
    }

    const goToPage = (page: number) => {
        if (activeTab === 'user') {
            setUserPage(page)
            return
        }
        setTeamPage(page)
    }

    return (
        <div className="space-y-6">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b pb-4">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">排行榜</h1>
                    <p className="text-gray-500 text-sm">看看谁才是最强练习者</p>
                </div>

                <div className="flex flex-col items-stretch md:items-end gap-3">
                    <div className="flex bg-gray-100 p-1 rounded-lg">
                        <button
                            onClick={() => setActiveTab('user')}
                            className={`px-4 py-1.5 rounded-md text-sm font-bold transition ${activeTab === 'user' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                        >
                            个人排行
                        </button>
                        <button
                            onClick={() => setActiveTab('team')}
                            className={`px-4 py-1.5 rounded-md text-sm font-bold transition ${activeTab === 'team' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                        >
                            团队排行
                        </button>
                    </div>

                    {activeTab === 'user' && (
                        <div className="w-full md:w-auto flex gap-2">
                            <input
                                value={searchInput}
                                onChange={(e) => setSearchInput(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === 'Enter') {
                                        handleSearchUser()
                                    }
                                }}
                                placeholder="检索用户名并置顶"
                                className="w-full md:w-56 px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                            <button
                                onClick={handleSearchUser}
                                disabled={searchLoading}
                                className="px-3 py-2 text-sm font-bold text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50"
                            >
                                {searchLoading ? '检索中' : '搜索'}
                            </button>
                        </div>
                    )}
                </div>
            </div>

            {error && <div className="p-4 bg-red-50 text-red-600 rounded-lg">{error}</div>}
            {activeTab === 'user' && searchError && <div className="p-3 bg-amber-50 text-amber-700 rounded-lg text-sm">{searchError}</div>}

            {activeTab === 'user' && pinnedUser && (
                <div className="rounded-xl border border-dashed border-blue-200 bg-blue-50/50 p-4">
                    <div className="flex items-center justify-between gap-4">
                        <div>
                            <p className="text-xs uppercase tracking-wider text-blue-500 font-black">已选中用户</p>
                            <div className="mt-1 flex items-center gap-3">
                                <Link to={`/profile/${pinnedUser.username}`} className="font-bold text-gray-900 hover:text-blue-600">
                                    {pinnedUser.display_name || pinnedUser.username}
                                </Link>
                                <span className="text-xs text-gray-500">@{pinnedUser.username}</span>
                                <span className="text-xs font-black text-blue-600">第 {pinnedUser.rank} 名</span>
                            </div>
                        </div>
                        <button
                            onClick={clearPinnedUser}
                            className="px-3 py-1.5 text-xs font-bold text-gray-600 bg-white border rounded-lg hover:bg-gray-50"
                        >
                            移除
                        </button>
                    </div>
                </div>
            )}

            <div className="relative bg-white border rounded-xl overflow-hidden shadow-sm min-h-[400px] flex flex-col pb-8">
                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse min-w-[600px]">
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
                                    <tr
                                        key={entry.rank}
                                        className={`transition-colors ${pinnedUser?.username === entry.username ? 'bg-blue-50' : 'hover:bg-gray-50'}`}
                                    >
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
                </div>

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

                <div className="absolute right-4 bottom-3">
                    <p className="text-xs text-gray-400">tips: 榜单为延迟榜单，数据可能存在短暂延迟，请耐心等待。</p>
                </div>
            </div>
            {!loading && (
                <Pagination
                    total={currentTotal}
                    page={currentPage}
                    pageSize={currentPageSize}
                    onPageChange={goToPage}
                    onPageSizeChange={(size) => {
                        if (activeTab === 'user') {
                            setUserPageSize(size)
                            return
                        }
                        setTeamPageSize(size)
                    }}
                />
            )}

        </div>
    )
}
