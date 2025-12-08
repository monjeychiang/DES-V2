import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { TrendingUp, TrendingDown, Wallet, ShieldCheck, Zap, Clock } from 'lucide-react'
import { balanceApi, strategyApi, orderApi } from '@/utils/api'
import { cn } from '@/utils/helpers'
import type { Strategy, Order, Balance } from '@/types'

interface StatCardProps {
    title: string
    value: string | number
    icon: React.ReactNode
    trend?: 'up' | 'down' | 'neutral'
    trendValue?: string
}

function StatCard({ title, value, icon, trend, trendValue }: StatCardProps) {
    return (
        <div className="bg-card rounded-xl border p-6">
            <div className="flex items-center justify-between mb-4">
                <span className="text-muted-foreground text-sm font-medium">{title}</span>
                <div className="p-2 bg-primary/10 rounded-lg">{icon}</div>
            </div>
            <div className="text-2xl font-bold">{value}</div>
            {trendValue && (
                <div className={cn(
                    'text-sm mt-1 flex items-center gap-1',
                    trend === 'up' && 'text-green-500',
                    trend === 'down' && 'text-red-500',
                    trend === 'neutral' && 'text-muted-foreground'
                )}>
                    {trend === 'up' && <TrendingUp className="w-4 h-4" />}
                    {trend === 'down' && <TrendingDown className="w-4 h-4" />}
                    {trendValue}
                </div>
            )}
        </div>
    )
}

function ActiveStrategies({ strategies }: { strategies: Strategy[] }) {
    const { t } = useTranslation()
    const active = strategies.filter(s => s.is_active)

    return (
        <div className="bg-card rounded-xl border p-6">
            <h3 className="font-semibold mb-4 flex items-center gap-2">
                <Zap className="w-5 h-5 text-primary" />
                {t('dashboard.activeStrategies')}
            </h3>
            {active.length === 0 ? (
                <p className="text-muted-foreground text-sm">No active strategies</p>
            ) : (
                <div className="space-y-3">
                    {active.slice(0, 5).map(s => (
                        <div key={s.id} className="flex items-center justify-between py-2 border-b last:border-0">
                            <div>
                                <div className="font-medium">{s.name}</div>
                                <div className="text-xs text-muted-foreground">{s.symbol}</div>
                            </div>
                            <span className="px-2 py-1 bg-green-100 text-green-700 text-xs rounded-full font-medium">
                                {t('strategy.statusActive')}
                            </span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}

function RecentOrders({ orders }: { orders: Order[] }) {
    const { t } = useTranslation()

    return (
        <div className="bg-card rounded-xl border p-6">
            <h3 className="font-semibold mb-4 flex items-center gap-2">
                <Clock className="w-5 h-5 text-primary" />
                {t('dashboard.recentOrders')}
            </h3>
            {orders.length === 0 ? (
                <p className="text-muted-foreground text-sm">No recent orders</p>
            ) : (
                <div className="space-y-3">
                    {orders.slice(0, 5).map(o => (
                        <div key={o.ID} className="flex items-center justify-between py-2 border-b last:border-0">
                            <div>
                                <div className="font-medium">{o.Symbol}</div>
                                <div className="text-xs text-muted-foreground">
                                    {new Date(o.CreatedAt).toLocaleTimeString()}
                                </div>
                            </div>
                            <div className="text-right">
                                <span className={cn(
                                    'font-semibold text-sm',
                                    o.Side === 'BUY' ? 'text-green-600' : 'text-red-600'
                                )}>
                                    {o.Side}
                                </span>
                                <div className="text-xs text-muted-foreground">{o.Qty} @ {o.Price}</div>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}

export default function DashboardPage() {
    const { t } = useTranslation()

    const { data: balanceData } = useQuery({
        queryKey: ['balance'],
        queryFn: () => balanceApi.get(),
        refetchInterval: 5000,
    })

    const { data: strategiesData } = useQuery({
        queryKey: ['strategies'],
        queryFn: () => strategyApi.getAll(),
        refetchInterval: 5000,
    })

    const { data: ordersData } = useQuery({
        queryKey: ['orders'],
        queryFn: () => orderApi.getAll(),
        refetchInterval: 5000,
    })

    const balance = balanceData?.data as Balance | undefined
    const strategies = (strategiesData?.data || []) as Strategy[]
    const orders = (ordersData?.data || []) as Order[]
    const activeCount = strategies.filter(s => s.is_active).length

    return (
        <div className="space-y-6">
            <h1 className="text-3xl font-bold">{t('dashboard.title')}</h1>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <StatCard
                    title={t('dashboard.totalBalance')}
                    value={`$${(balance?.total || 0).toLocaleString()}`}
                    icon={<Wallet className="w-5 h-5 text-primary" />}
                />
                <StatCard
                    title={t('dashboard.availableBalance')}
                    value={`$${(balance?.available || 0).toLocaleString()}`}
                    icon={<ShieldCheck className="w-5 h-5 text-primary" />}
                />
                <StatCard
                    title={t('dashboard.margin')}
                    value={`$${(balance?.margin || 0).toLocaleString()}`}
                    icon={<TrendingUp className="w-5 h-5 text-primary" />}
                />
                <StatCard
                    title={t('dashboard.activeStrategies')}
                    value={activeCount}
                    icon={<Zap className="w-5 h-5 text-primary" />}
                />
            </div>

            {/* Widgets Grid */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <ActiveStrategies strategies={strategies} />
                <RecentOrders orders={orders} />
            </div>
        </div>
    )
}
