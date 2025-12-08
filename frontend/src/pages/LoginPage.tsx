import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import { authApi } from '@/utils/api'

export default function LoginPage() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const login = useAuthStore((state) => state.login)

    const [mode, setMode] = useState<'login' | 'register'>('login')
    const [email, setEmail] = useState('')
    const [password, setPassword] = useState('')
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState('')

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setLoading(true)
        setError('')

        try {
            const fn = mode === 'login' ? authApi.login : authApi.register
            const res = await fn(email, password)
            const token = res.data?.token

            if (token) {
                login(token, res.data?.user)
                navigate('/')
            } else {
                setError('No token received')
            }
        } catch (err: unknown) {
            const error = err as { response?: { data?: { error?: string } }; message?: string }
            setError(error.response?.data?.error || error.message || 'Login failed')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="w-full max-w-md">
            <div className="mb-8 text-center">
                <h2 className="text-2xl font-bold mb-2">
                    {mode === 'login' ? t('auth.loginTitle') : t('auth.registerTitle')}
                </h2>
                <p className="text-muted-foreground text-sm">
                    {mode === 'login' ? t('auth.loginSubtitle') : t('auth.registerSubtitle')}
                </p>
            </div>

            {/* Toggle */}
            <div className="flex p-1 bg-muted rounded-lg mb-6">
                <button
                    type="button"
                    className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${mode === 'login' ? 'bg-background shadow' : 'text-muted-foreground'
                        }`}
                    onClick={() => setMode('login')}
                >
                    {t('auth.login')}
                </button>
                <button
                    type="button"
                    className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${mode === 'register' ? 'bg-background shadow' : 'text-muted-foreground'
                        }`}
                    onClick={() => setMode('register')}
                >
                    {t('auth.register')}
                </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                    <label className="block text-sm font-medium mb-1.5">{t('auth.email')}</label>
                    <input
                        type="email"
                        className="w-full px-4 py-2 border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                        placeholder="user@example.com"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        required
                    />
                </div>

                <div>
                    <label className="block text-sm font-medium mb-1.5">{t('auth.password')}</label>
                    <input
                        type="password"
                        className="w-full px-4 py-2 border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                        placeholder="••••••••"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        required
                        minLength={6}
                    />
                </div>

                {error && (
                    <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-lg text-destructive text-sm">
                        {error}
                    </div>
                )}

                <button
                    type="submit"
                    disabled={loading}
                    className="w-full py-3 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
                >
                    {loading ? t('common.loading') : mode === 'login' ? t('auth.login') : t('auth.register')}
                </button>
            </form>
        </div>
    )
}
