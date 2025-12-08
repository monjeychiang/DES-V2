import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useThemeStore } from '@/stores/themeStore'
import { connectionApi } from '@/utils/api'
import type { Connection } from '@/types'
import {
    Wifi,
    Shield,
    Bell,
    Palette,
    Plus,
    Trash2,
    TestTube,
    Languages,
    Moon,
    Sun
} from 'lucide-react'
import { cn } from '@/utils/helpers'

type TabKey = 'connections' | 'risk' | 'notifications' | 'appearance'

interface TabItem {
    key: TabKey
    icon: React.ReactNode
    label: string
}

const tabs: TabItem[] = [
    { key: 'connections', icon: <Wifi className="w-4 h-4" />, label: 'settings.connections' },
    { key: 'risk', icon: <Shield className="w-4 h-4" />, label: 'settings.riskManagement' },
    { key: 'notifications', icon: <Bell className="w-4 h-4" />, label: 'settings.notifications' },
    { key: 'appearance', icon: <Palette className="w-4 h-4" />, label: 'settings.appearance' },
]

function ConnectionsTab() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [showForm, setShowForm] = useState(false)
    const [formData, setFormData] = useState({ name: '', api_key: '', api_secret: '', exchange: 'binance', venue: 'FUTURES' })

    const { data, isLoading } = useQuery({
        queryKey: ['connections'],
        queryFn: () => connectionApi.getAll(),
    })

    const createMutation = useMutation({
        mutationFn: (data: typeof formData) => connectionApi.create(data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['connections'] })
            setShowForm(false)
            setFormData({ name: '', api_key: '', api_secret: '', exchange: 'binance', venue: 'FUTURES' })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (id: string) => connectionApi.delete(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['connections'] })
        },
    })

    const testMutation = useMutation({
        mutationFn: (id: string) => connectionApi.test(id),
    })

    const connections = (data?.data || []) as Connection[]

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-lg font-semibold">{t('settings.connections')}</h3>
                    <p className="text-sm text-muted-foreground">Manage your exchange API connections</p>
                </div>
                <button
                    onClick={() => setShowForm(true)}
                    className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90"
                >
                    <Plus className="w-4 h-4" /> Add Connection
                </button>
            </div>

            {showForm && (
                <div className="bg-muted/50 rounded-xl border p-6 space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">Name</label>
                            <input
                                type="text"
                                value={formData.name}
                                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                                className="w-full px-3 py-2 border rounded-lg bg-background"
                                placeholder="My Binance"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Exchange</label>
                            <select
                                value={formData.exchange}
                                onChange={(e) => setFormData({ ...formData, exchange: e.target.value })}
                                className="w-full px-3 py-2 border rounded-lg bg-background"
                            >
                                <option value="binance">Binance</option>
                            </select>
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">API Key</label>
                            <input
                                type="text"
                                value={formData.api_key}
                                onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
                                className="w-full px-3 py-2 border rounded-lg bg-background font-mono text-sm"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">API Secret</label>
                            <input
                                type="password"
                                value={formData.api_secret}
                                onChange={(e) => setFormData({ ...formData, api_secret: e.target.value })}
                                className="w-full px-3 py-2 border rounded-lg bg-background"
                            />
                        </div>
                    </div>
                    <div className="flex gap-2">
                        <button
                            onClick={() => createMutation.mutate(formData)}
                            disabled={createMutation.isPending}
                            className="px-4 py-2 bg-primary text-primary-foreground rounded-lg"
                        >
                            {createMutation.isPending ? 'Saving...' : t('common.save')}
                        </button>
                        <button
                            onClick={() => setShowForm(false)}
                            className="px-4 py-2 bg-muted rounded-lg"
                        >
                            {t('common.cancel')}
                        </button>
                    </div>
                </div>
            )}

            {isLoading ? (
                <div className="text-center py-8 text-muted-foreground">{t('common.loading')}</div>
            ) : connections.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">No connections yet</div>
            ) : (
                <div className="space-y-3">
                    {connections.map((c) => (
                        <div key={c.id} className="flex items-center justify-between p-4 bg-card rounded-xl border">
                            <div>
                                <div className="font-medium">{c.name}</div>
                                <div className="text-sm text-muted-foreground">{c.exchange} • {c.venue}</div>
                            </div>
                            <div className="flex items-center gap-2">
                                <button
                                    onClick={() => testMutation.mutate(c.id)}
                                    className="p-2 hover:bg-accent rounded-lg text-muted-foreground"
                                >
                                    <TestTube className="w-4 h-4" />
                                </button>
                                <button
                                    onClick={() => deleteMutation.mutate(c.id)}
                                    className="p-2 hover:bg-red-100 rounded-lg text-red-600"
                                >
                                    <Trash2 className="w-4 h-4" />
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}

function AppearanceTab() {
    const { t, i18n } = useTranslation()
    const { theme, toggleTheme } = useThemeStore()

    const toggleLanguage = () => {
        const newLang = i18n.language === 'zh-TW' ? 'en' : 'zh-TW'
        i18n.changeLanguage(newLang)
    }

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-semibold">{t('settings.appearance')}</h3>
                <p className="text-sm text-muted-foreground">Customize the look and feel</p>
            </div>

            <div className="space-y-4">
                <div className="flex items-center justify-between p-4 bg-card rounded-xl border">
                    <div className="flex items-center gap-3">
                        {theme === 'light' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
                        <div>
                            <div className="font-medium">{t('settings.darkMode')}</div>
                            <div className="text-sm text-muted-foreground">
                                Currently: {theme === 'light' ? 'Light' : 'Dark'}
                            </div>
                        </div>
                    </div>
                    <button
                        onClick={toggleTheme}
                        className={cn(
                            'relative w-12 h-6 rounded-full transition-colors',
                            theme === 'dark' ? 'bg-primary' : 'bg-muted'
                        )}
                    >
                        <div className={cn(
                            'absolute top-1 w-4 h-4 rounded-full bg-white transition-transform',
                            theme === 'dark' ? 'translate-x-7' : 'translate-x-1'
                        )} />
                    </button>
                </div>

                <div className="flex items-center justify-between p-4 bg-card rounded-xl border">
                    <div className="flex items-center gap-3">
                        <Languages className="w-5 h-5" />
                        <div>
                            <div className="font-medium">{t('settings.language')}</div>
                            <div className="text-sm text-muted-foreground">
                                Currently: {i18n.language === 'zh-TW' ? '繁體中文' : 'English'}
                            </div>
                        </div>
                    </div>
                    <button
                        onClick={toggleLanguage}
                        className="px-4 py-2 bg-muted rounded-lg text-sm font-medium hover:bg-accent"
                    >
                        {i18n.language === 'zh-TW' ? 'English' : '繁體中文'}
                    </button>
                </div>
            </div>
        </div>
    )
}

function RiskTab() {
    const { t } = useTranslation()

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-semibold">{t('settings.riskManagement')}</h3>
                <p className="text-sm text-muted-foreground">Configure global risk parameters</p>
            </div>
            <div className="bg-card rounded-xl border p-6 text-center text-muted-foreground">
                Risk management settings coming soon...
            </div>
        </div>
    )
}

function NotificationsTab() {
    const { t } = useTranslation()

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-semibold">{t('settings.notifications')}</h3>
                <p className="text-sm text-muted-foreground">Configure alerts and notifications</p>
            </div>
            <div className="bg-card rounded-xl border p-6 text-center text-muted-foreground">
                Notification settings coming soon...
            </div>
        </div>
    )
}

export default function SettingsPage() {
    const { t } = useTranslation()
    const [activeTab, setActiveTab] = useState<TabKey>('connections')

    return (
        <div className="flex gap-6">
            {/* Sidebar Tabs */}
            <div className="w-64 shrink-0">
                <h1 className="text-3xl font-bold mb-6">{t('settings.title')}</h1>
                <nav className="space-y-1">
                    {tabs.map((tab) => (
                        <button
                            key={tab.key}
                            onClick={() => setActiveTab(tab.key)}
                            className={cn(
                                'w-full flex items-center gap-3 px-4 py-3 rounded-lg text-left transition-colors',
                                activeTab === tab.key
                                    ? 'bg-primary text-primary-foreground'
                                    : 'text-muted-foreground hover:bg-accent'
                            )}
                        >
                            {tab.icon}
                            <span className="font-medium">{t(tab.label)}</span>
                        </button>
                    ))}
                </nav>
            </div>

            {/* Content */}
            <div className="flex-1">
                {activeTab === 'connections' && <ConnectionsTab />}
                {activeTab === 'risk' && <RiskTab />}
                {activeTab === 'notifications' && <NotificationsTab />}
                {activeTab === 'appearance' && <AppearanceTab />}
            </div>
        </div>
    )
}
