import { useTranslation } from 'react-i18next'
import { useThemeStore } from '@/stores/themeStore'
import { useWebSocketStore } from '@/hooks/useWebSocket'
import { Moon, Sun, Globe, Bell, User, Wifi, WifiOff } from 'lucide-react'
import { cn } from '@/utils/helpers'

export default function Header() {
    const { i18n, t } = useTranslation()
    const { theme, toggleTheme } = useThemeStore()
    const isWsConnected = useWebSocketStore((state) => state.isConnected)

    const toggleLanguage = () => {
        const newLang = i18n.language === 'zh-TW' ? 'en' : 'zh-TW'
        i18n.changeLanguage(newLang)
    }

    return (
        <header className="h-14 lg:h-16 border-b bg-card flex items-center justify-between px-4 lg:px-6">
            {/* Spacer for mobile menu button */}
            <div className="w-10 lg:hidden" />

            {/* Search (hidden on mobile) */}
            <div className="hidden md:block flex-1 max-w-md">
                <input
                    type="text"
                    placeholder={`${t('common.search')}...`}
                    className="w-full px-4 py-2 bg-muted rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                />
            </div>

            {/* Actions */}
            <div className="flex items-center gap-1 lg:gap-2">
                {/* WebSocket Status */}
                <div className={cn(
                    'p-2 rounded-lg',
                    isWsConnected ? 'text-green-500' : 'text-muted-foreground'
                )} title={isWsConnected ? 'Connected' : 'Disconnected'}>
                    {isWsConnected ? <Wifi className="w-4 h-4" /> : <WifiOff className="w-4 h-4" />}
                </div>

                {/* Language Toggle */}
                <button
                    onClick={toggleLanguage}
                    className="p-2 rounded-lg hover:bg-accent transition-colors"
                    title={t('settings.language')}
                >
                    <Globe className="w-4 h-4 lg:w-5 lg:h-5 text-muted-foreground" />
                </button>

                {/* Theme Toggle */}
                <button
                    onClick={toggleTheme}
                    className="p-2 rounded-lg hover:bg-accent transition-colors"
                    title={t('settings.darkMode')}
                >
                    {theme === 'light' ? (
                        <Moon className="w-4 h-4 lg:w-5 lg:h-5 text-muted-foreground" />
                    ) : (
                        <Sun className="w-4 h-4 lg:w-5 lg:h-5 text-muted-foreground" />
                    )}
                </button>

                {/* Notifications */}
                <button className="p-2 rounded-lg hover:bg-accent transition-colors relative">
                    <Bell className="w-4 h-4 lg:w-5 lg:h-5 text-muted-foreground" />
                    <span className="absolute top-1 right-1 w-2 h-2 bg-destructive rounded-full" />
                </button>

                {/* User */}
                <button className="p-2 rounded-lg hover:bg-accent transition-colors">
                    <User className="w-4 h-4 lg:w-5 lg:h-5 text-muted-foreground" />
                </button>
            </div>
        </header>
    )
}
