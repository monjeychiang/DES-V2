import { useState, useEffect } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import {
    LayoutDashboard,
    Zap,
    ClipboardList,
    TrendingUp,
    Settings,
    LogOut,
    Activity,
    Menu,
    X,
} from 'lucide-react'
import { cn } from '@/utils/helpers'

const navItems = [
    { path: '/', icon: LayoutDashboard, labelKey: 'nav.dashboard' },
    { path: '/strategies', icon: Zap, labelKey: 'nav.strategies' },
    { path: '/orders', icon: ClipboardList, labelKey: 'nav.orders' },
    { path: '/performance', icon: TrendingUp, labelKey: 'nav.performance' },
    { path: '/settings', icon: Settings, labelKey: 'nav.settings' },
]

export default function Sidebar() {
    const { t } = useTranslation()
    const logout = useAuthStore((state) => state.logout)
    const location = useLocation()
    const [isOpen, setIsOpen] = useState(false)

    // Close sidebar on route change (mobile)
    useEffect(() => {
        setIsOpen(false)
    }, [location.pathname])

    return (
        <>
            {/* Mobile Toggle Button */}
            <button
                onClick={() => setIsOpen(!isOpen)}
                className="lg:hidden fixed top-4 left-4 z-50 p-2 bg-card border rounded-lg shadow-lg"
            >
                {isOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
            </button>

            {/* Backdrop for mobile */}
            {isOpen && (
                <div
                    className="lg:hidden fixed inset-0 bg-black/50 z-30"
                    onClick={() => setIsOpen(false)}
                />
            )}

            {/* Sidebar */}
            <aside className={cn(
                'fixed left-0 top-0 h-screen w-64 bg-card border-r flex flex-col z-40 transition-transform duration-200',
                isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'
            )}>
                {/* Logo */}
                <div className="p-6 border-b">
                    <div className="flex items-center gap-3">
                        <div className="w-10 h-10 bg-primary rounded-lg flex items-center justify-center">
                            <Activity className="w-6 h-6 text-primary-foreground" />
                        </div>
                        <div>
                            <h1 className="font-bold text-lg">DES Trading</h1>
                            <p className="text-xs text-muted-foreground">v2.0</p>
                        </div>
                    </div>
                </div>

                {/* Navigation */}
                <nav className="flex-1 p-4 space-y-1">
                    {navItems.map(({ path, icon: Icon, labelKey }) => (
                        <NavLink
                            key={path}
                            to={path}
                            end={path === '/'}
                            className={({ isActive }) =>
                                cn(
                                    'flex items-center gap-3 px-4 py-3 rounded-lg transition-colors',
                                    isActive
                                        ? 'bg-primary text-primary-foreground'
                                        : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                                )
                            }
                        >
                            <Icon className="w-5 h-5" />
                            <span className="font-medium">{t(labelKey)}</span>
                        </NavLink>
                    ))}
                </nav>

                {/* Logout */}
                <div className="p-4 border-t">
                    <button
                        onClick={logout}
                        className="flex items-center gap-3 px-4 py-3 w-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive rounded-lg transition-colors"
                    >
                        <LogOut className="w-5 h-5" />
                        <span className="font-medium">{t('auth.logout')}</span>
                    </button>
                </div>
            </aside>
        </>
    )
}
