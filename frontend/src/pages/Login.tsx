import { useState, useEffect, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/authStore'
import { authApi } from '@/api/endpoints/auth'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import styles from './Login.module.css'

export function Login() {
  const navigate = useNavigate()
  const login = useAuthStore((state) => state.login)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  useEffect(() => {
    const usernameInput = document.getElementById('username')
    usernameInput?.focus()
  }, [])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      const response = await authApi.login({ username, password })
      login(response.token, response.user)
      navigate('/')
    } catch {
      setError('Invalid username or password')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.header}>
          <div className={styles.logo}>
            <div className={styles.logoIcon}>
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 2L2 7L12 12L22 7L12 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                <path d="M2 17L12 22L22 17" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                <path d="M2 12L12 17L22 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </div>
          </div>
          <h1 className={styles.title}>DevOps Toolkit</h1>
          <p className={styles.subtitle}>Sign in to your account</p>
        </div>

        <form onSubmit={handleSubmit} className={styles.form}>
          <Input
            id="username"
            label="Username"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={isLoading}
            autoComplete="username"
            required
          />

          <Input
            id="password"
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={isLoading}
            autoComplete="current-password"
            required
          />

          {error && (
            <div className={styles.error}>
              {error}
            </div>
          )}

          <Button
            type="submit"
            variant="primary"
            size="lg"
            className={styles.submitButton}
            disabled={isLoading}
            loading={isLoading}
          >
            {isLoading ? 'Signing in...' : 'Sign in'}
          </Button>
        </form>
      </div>
    </div>
  )
}