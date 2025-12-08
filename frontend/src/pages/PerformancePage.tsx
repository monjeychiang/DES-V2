import { useTranslation } from 'react-i18next'
import {
    AreaChart,
    Area,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    ResponsiveContainer
} from 'recharts'
import { Calendar, TrendingUp, TrendingDown, Target, BarChart3 } from 'lucide-react'
import { cn, formatCurrency, formatPercent } from '@/utils/helpers'

// Mock data for demonstration
const mockPnlData = [
    { date: 'Jan', pnl: 1200 },
    { date: 'Feb', pnl: 1800 },
    { date: 'Mar', pnl: 1400 },
    { date: 'Apr', pnl: 2200 },
    { date: 'May', pnl: 1900 },
    { date: 'Jun', pnl: 2800 },
    { date: 'Jul', pnl: 3200 },
    { date: 'Aug', pnl: 2900 },
    { date: 'Sep', pnl: 3500 },
    { date: 'Oct', pnl: 4100 },
    { date: 'Nov', pnl: 3800 },
    { date: 'Dec', pnl: 4500 },
]

const mockDrawdownData = [
    { date: 'Jan', drawdown: -2 },
    { date: 'Feb', drawdown: -3 },
    { date: 'Mar', drawdown: -5 },
    { date: 'Apr', drawdown: -2 },
    { date: 'May', drawdown: -4 },
    { date: 'Jun', drawdown: -1 },
    { date: 'Jul', drawdown: -2 },
    { date: 'Aug', drawdown: -3 },
    { date: 'Sep', drawdown: -1 },
    { date: 'Oct', drawdown: -2 },
    { date: 'Nov', drawdown: -3 },
    { date: 'Dec', drawdown: -1 },
]

interface MetricCardProps {
    title: string
    value: string
    icon: React.ReactNode
    trend?: 'up' | 'down' | 'neutral'
}

function MetricCard({ title, value, icon, trend }: MetricCardProps) {
    return (
        <div className="bg-card rounded-xl border p-5">
            <div className="flex items-center gap-3 text-muted-foreground mb-2">
                {icon}
                <span className="text-xs font-semibold uppercase">{title}</span>
            </div>
            <div className={cn(
                'text-2xl font-bold',
                trend === 'up' && 'text-green-600',
                trend === 'down' && 'text-red-600'
            )}>
                {value}
            </div>
        </div>
    )
}

export default function PerformancePage() {
    const { t } = useTranslation()

    // Mock metrics
    const totalPnl = 4500
    const winRate = 65.3
    const maxDrawdown = -8.2
    const sharpeRatio = 1.45
    const totalTrades = 128

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <h1 className="text-3xl font-bold">{t('performance.title')}</h1>
                <div className="flex items-center gap-2 px-4 py-2 bg-muted rounded-lg">
                    <Calendar className="w-4 h-4 text-muted-foreground" />
                    <span className="text-sm">Last 12 Months</span>
                </div>
            </div>

            {/* Metrics Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
                <MetricCard
                    title={t('performance.realizedPnl')}
                    value={formatCurrency(totalPnl)}
                    icon={<TrendingUp className="w-4 h-4" />}
                    trend="up"
                />
                <MetricCard
                    title={t('performance.winRate')}
                    value={formatPercent(winRate)}
                    icon={<Target className="w-4 h-4" />}
                    trend="up"
                />
                <MetricCard
                    title={t('performance.maxDrawdown')}
                    value={formatPercent(maxDrawdown)}
                    icon={<TrendingDown className="w-4 h-4" />}
                    trend="down"
                />
                <MetricCard
                    title={t('performance.sharpeRatio')}
                    value={sharpeRatio.toFixed(2)}
                    icon={<BarChart3 className="w-4 h-4" />}
                />
                <MetricCard
                    title={t('performance.totalTrades')}
                    value={totalTrades.toString()}
                    icon={<Target className="w-4 h-4" />}
                />
            </div>

            {/* Charts */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* PnL Chart */}
                <div className="bg-card rounded-xl border p-6">
                    <h3 className="font-semibold mb-4">{t('performance.realizedPnl')} Curve</h3>
                    <ResponsiveContainer width="100%" height={300}>
                        <AreaChart data={mockPnlData}>
                            <defs>
                                <linearGradient id="colorPnl" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                                    <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                                </linearGradient>
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                            <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={12} />
                            <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} />
                            <Tooltip
                                contentStyle={{
                                    backgroundColor: 'hsl(var(--card))',
                                    border: '1px solid hsl(var(--border))',
                                    borderRadius: '8px',
                                }}
                            />
                            <Area
                                type="monotone"
                                dataKey="pnl"
                                stroke="#10b981"
                                strokeWidth={2}
                                fillOpacity={1}
                                fill="url(#colorPnl)"
                            />
                        </AreaChart>
                    </ResponsiveContainer>
                </div>

                {/* Drawdown Chart */}
                <div className="bg-card rounded-xl border p-6">
                    <h3 className="font-semibold mb-4">{t('performance.maxDrawdown')} History</h3>
                    <ResponsiveContainer width="100%" height={300}>
                        <AreaChart data={mockDrawdownData}>
                            <defs>
                                <linearGradient id="colorDrawdown" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3} />
                                    <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                                </linearGradient>
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                            <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" fontSize={12} />
                            <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} />
                            <Tooltip
                                contentStyle={{
                                    backgroundColor: 'hsl(var(--card))',
                                    border: '1px solid hsl(var(--border))',
                                    borderRadius: '8px',
                                }}
                            />
                            <Area
                                type="monotone"
                                dataKey="drawdown"
                                stroke="#ef4444"
                                strokeWidth={2}
                                fillOpacity={1}
                                fill="url(#colorDrawdown)"
                            />
                        </AreaChart>
                    </ResponsiveContainer>
                </div>
            </div>

            {/* Monthly Returns Table */}
            <div className="bg-card rounded-xl border p-6">
                <h3 className="font-semibold mb-4">Monthly Returns</h3>
                <div className="grid grid-cols-12 gap-2">
                    {mockPnlData.map((m, i) => {
                        const change = i > 0 ? ((m.pnl - mockPnlData[i - 1].pnl) / mockPnlData[i - 1].pnl * 100) : 0
                        return (
                            <div key={m.date} className="text-center">
                                <div className="text-xs text-muted-foreground mb-1">{m.date}</div>
                                <div className={cn(
                                    'py-2 rounded text-sm font-medium',
                                    change >= 0 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                                )}>
                                    {change >= 0 ? '+' : ''}{change.toFixed(1)}%
                                </div>
                            </div>
                        )
                    })}
                </div>
            </div>
        </div>
    )
}
