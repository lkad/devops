import { useState } from 'react'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { useToast } from '@/components/ui/Toast'
import styles from './Settings.module.css'

interface ProfileForm {
  username: string
  email: string
  fullName: string
  role: string
}

interface NotificationPreference {
  id: string
  label: string
  enabled: boolean
}

export function Settings() {
  const { user } = useAuthStore()
  const { addToast } = useToast()

  const [profile, setProfile] = useState<ProfileForm>({
    username: user?.username || '',
    email: '',
    fullName: '',
    role: user?.roles?.[0] || '',
  })

  const [notifications, setNotifications] = useState<NotificationPreference[]>([
    { id: '1', label: 'Email notifications', enabled: true },
    { id: '2', label: 'Push notifications', enabled: false },
    { id: '3', label: 'Weekly digest', enabled: true },
    { id: '4', label: 'Security alerts', enabled: true },
  ])

  const [isSaving, setIsSaving] = useState(false)

  const handleProfileSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSaving(true)
    try {
      // Simulate API call
      await new Promise(resolve => setTimeout(resolve, 500))
      addToast({ type: 'success', message: 'Profile updated successfully' })
    } catch {
      addToast({ type: 'error', message: 'Failed to update profile' })
    } finally {
      setIsSaving(false)
    }
  }

  const toggleNotification = (id: string) => {
    setNotifications(prev =>
      prev.map(n => n.id === id ? { ...n, enabled: !n.enabled } : n)
    )
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Settings</h1>
      </div>

      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Profile</h2>
        <Card>
          <form onSubmit={handleProfileSubmit}>
            <div className={styles.formGrid}>
              <div className={styles.formField}>
                <label className={styles.formLabel}>Username</label>
                <input
                  type="text"
                  value={profile.username}
                  onChange={(e) => setProfile({ ...profile, username: e.target.value })}
                  className={styles.formInput}
                  disabled
                />
              </div>
              <div className={styles.formField}>
                <label className={styles.formLabel}>Email</label>
                <input
                  type="email"
                  value={profile.email}
                  onChange={(e) => setProfile({ ...profile, email: e.target.value })}
                  className={styles.formInput}
                  placeholder="your@email.com"
                />
              </div>
              <div className={styles.formField}>
                <label className={styles.formLabel}>Full Name</label>
                <input
                  type="text"
                  value={profile.fullName}
                  onChange={(e) => setProfile({ ...profile, fullName: e.target.value })}
                  className={styles.formInput}
                  placeholder="Your Name"
                />
              </div>
              <div className={styles.formField}>
                <label className={styles.formLabel}>Role</label>
                <input
                  type="text"
                  value={profile.role}
                  onChange={(e) => setProfile({ ...profile, role: e.target.value })}
                  className={styles.formInput}
                  disabled
                />
              </div>
            </div>
            <div className={styles.formActions}>
              <Button variant="primary" type="submit" loading={isSaving}>
                Save Profile
              </Button>
            </div>
          </form>
        </Card>
      </div>

      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Notifications</h2>
        <Card>
          <div className={styles.notificationsList}>
            {notifications.map((notif) => (
              <div key={notif.id} className={styles.toggle}>
                <span className={styles.toggleLabel}>{notif.label}</span>
                <div
                  className={`${styles.toggleSwitch} ${notif.enabled ? styles.toggleSwitchActive : ''}`}
                  onClick={() => toggleNotification(notif.id)}
                >
                  <div className={styles.toggleKnob} />
                </div>
              </div>
            ))}
          </div>
        </Card>
      </div>

      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Preferences</h2>
        <Card>
          <div className={styles.formGrid}>
            <div className={styles.formField}>
              <label className={styles.formLabel}>Timezone</label>
              <select className={styles.formSelect} defaultValue="UTC">
                <option value="UTC">UTC</option>
                <option value="America/New_York">America/New_York</option>
                <option value="America/Los_Angeles">America/Los_Angeles</option>
                <option value="Europe/London">Europe/London</option>
                <option value="Asia/Shanghai">Asia/Shanghai</option>
              </select>
            </div>
            <div className={styles.formField}>
              <label className={styles.formLabel}>Language</label>
              <select className={styles.formSelect} defaultValue="en">
                <option value="en">English</option>
                <option value="zh">中文</option>
              </select>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}