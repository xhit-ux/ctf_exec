import React from 'react'
import { UserProfile } from '../../types/profile'
import { ProfileSidebar } from './ProfileSidebar'

type ProfileRouteView = 'profile' | 'followers' | 'following' | 'edit'

interface ProfileChromeProps {
  profile: UserProfile | null
  currentView: ProfileRouteView
  onFollowChange?: (isFollowing: boolean, newCount?: number) => void
	onAvatarEdit?: () => void
  heading?: string
  description?: string
  children: React.ReactNode
}

export default function ProfileChrome({
	profile,
	currentView,
	onFollowChange,
	onAvatarEdit,
	heading,
	description,
	children,
}: ProfileChromeProps) {
	void heading
	void description
	return (
		<div className="w-full pb-4">
			<div className="grid items-start gap-6 xl:grid-cols-[340px,minmax(0,1fr)]">
				<ProfileSidebar
					profile={profile}
					onFollowChange={onFollowChange}
					showOwnEditButton={currentView !== 'edit'}
					onAvatarEdit={onAvatarEdit}
				/>
				<div className="min-w-0">{children}</div>
			</div>
		</div>
	)
}
