import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    ArrowLeft,
    Play,
    Pause,
    Square,
    Zap,
    Settings,
    Activity,
    TrendingUp,
    Clock,
    Target
} from 'lucide-react'
import { strategyApi } from '@/utils/api'
import { cn, formatCurrency, formatPercent } from '@/utils/helpers'
import type { Strategy, StrategyPerformance } from '@/types'

export default function StrategyDetailPage() {
    const { id } = useParams<{ id: string }>()
    const navigate = useNavigate()
    const { t } = useTranslation()
    const queryClient = useQueryClient()

    const { data: strategiesData, isLoading } = useQuery({
        queryKey: ['strategies'],
        queryFn: () => strategyApi.getAll(),
    })

    const { data: perfData } = useQuery({
        queryKey: ['strategy-performance', id],
        queryFn: () => strategyApi.getPerformance(id!),
        enabled: !!id,
    })

    const actionMutation = useMutation({
        mutationFn: async (action: string) => {
            switch (action) {
                case 'start': return strategyApi.start(id!)
                case 'pause': return strategyApi.pause(id!)
                case 'stop': return strategyApi.stop(id!)
                case 'panic': return strategyApi.panicSell(id!)
                default: throw new Error('Unknown action')
            }
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['strategies'] })
            queryClient.invalidateQueries({ queryKey: ['strategy-performance', id] })
        },
    })

    const strategies = (strategiesData?.data || []) as Strategy[]
    const strategy = strategies.find(s => s.id === id)
    const performance = perfData?.data as StrategyPerformance | undefined

    const handleAction = (action: string) => {
        if (action === 'panic') {
            if (!confirm(t('strategy.panicSell') + '?')) return
        }
        actionMutation.mutate(action)
    }

    if (isLoading) {
        return <div className="p-8 text-center text-muted-foreground">{t('common.loading')}</div>
    }

    if (!strategy) {
        return (
            <div className="p-8 text-center">
                <p className="text-muted-foreground mb-4">Strategy not found</p>
                <button onClick={() => navigate('/strategies')} className="text-primary hover:underline">
                    Back to strategies
                </button>
            </div>
        )
    }

    const isActive = strategy.is_active
    const pnl = performance?.realized_pnl || 0

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center gap-4">
                <button
                    onClick={() => navigate('/strategies')}
                    className="p-2 hover:bg-accent rounded-full"
                >
                    <ArrowLeft className="w-5 h-5" />
                </button>

                <div className="flex-1">
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold">{strategy.name}</h1>
                        <span className={cn(
                            'px-3 py-1 rounded-full text-sm font-medium',
                            isActive ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-700'
                        )}>
                            {strategy.status}
                        </span>
                    </div>
                    <p className="text-muted-foreground text-sm mt-1">
                        {strategy.symbol} • {strategy.type}
                    </p>
                </div>

                <div className="flex items-center gap-2">
                    {isActive ? (
                        <>
                            <button
                                onClick={() => handleAction('pause')}
                                className="flex items-center gap-2 px-4 py-2 bg-yellow-100 text-yellow-700 rounded-lg hover:bg-yellow-200"
                            >
                                <Pause className="w-4 h-4" /> {t('strategy.pause')}
                            </button>
                            <button
                                onClick={() => handleAction('stop')}
                                className="flex items-center gap-2 px-4 py-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200"
                            >
                                <Square className="w-4 h-4" /> {t('strategy.stop')}
                            </button>
                            <button
                                onClick={() => handleAction('panic')}
                                className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                            >
                                <Zap className="w-4 h-4" /> {t('strategy.panicSell')}
                            </button>
                        </>
                    ) : (
                        <button
                            onClick={() => handleAction('start')}
                            disabled={!strategy.connection_id}
                            className="flex items-center gap-2 px-6 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
                        >
                            <Play className="w-4 h-4" /> {t('strategy.start')}
                        </button>
                    )}
                    <button className="p-2 hover:bg-accent rounded-lg">
                        <Settings className="w-5 h-5 text-muted-foreground" />
                    </button>
                </div>
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <div className="bg-card rounded-xl border p-5">
                    <div className="flex items-center gap-3 text-muted-foreground mb-2">
                        <Activity className="w-4 h-4" />
                        <span className="text-xs font-semibold uppercase">{t('performance.realizedPnl')}</span>
                    </div>
                    <div className={cn('text-2xl font-bold', pnl >= 0 ? 'text-green-600' : 'text-red-600')}>
                        {formatCurrency(pnl)}
                    </div>
                </div>

                <div className="bg-card rounded-xl border p-5">
                    <div className="flex items-center gap-3 text-muted-foreground mb-2">
                        <TrendingUp className="w-4 h-4" />
                        <span className="text-xs font-semibold uppercase">{t('performance.totalTrades')}</span>
                    </div>
                    <div className="text-2xl font-bold">
                        {performance?.total_trades || 0}
                    </div>
                </div>

                <div className="bg-card rounded-xl border p-5">
                    <div className="flex items-center gap-3 text-muted-foreground mb-2">
                        <Target className="w-4 h-4" />
                        <span className="text-xs font-semibold uppercase">{t('performance.winRate')}</span>
                    </div>
                    <div className="text-2xl font-bold">
                        {formatPercent(performance?.win_rate || 0)}
                    </div>
                </div>

                <div className="bg-card rounded-xl border p-5">
                    <div className="flex items-center gap-3 text-muted-foreground mb-2">
                        <Clock className="w-4 h-4" />
                        <span className="text-xs font-semibold uppercase">Interval</span>
                    </div>
                    <div className="text-2xl font-bold">
                        {strategy.interval || '--'}
                    </div>
                </div>
            </div>

            {/* Charts & Params */}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Chart Placeholder */}
                <div className="lg:col-span-2 bg-card rounded-xl border h-96 flex items-center justify-center text-muted-foreground">
                    <div className="text-center">
                        <Activity className="w-12 h-12 mx-auto mb-2 opacity-20" />
                        <p>Performance Chart (Coming Soon)</p>
                    </div>
                </div>

                {/* Params */}
                <div className="bg-card rounded-xl border p-6">
                    <h3 className="font-bold mb-4">Parameters</h3>
                    <div className="space-y-3">
                        <div className="flex justify-between py-2 border-b">
                            <span className="text-muted-foreground text-sm">Symbol</span>
                            <span className="font-mono text-sm">{strategy.symbol}</span>
                        </div>
                        <div className="flex justify-between py-2 border-b">
                            <span className="text-muted-foreground text-sm">Type</span>
                            <span className="font-mono text-sm">{strategy.type}</span>
                        </div>
                        <div className="flex justify-between py-2 border-b">
                            <span className="text-muted-foreground text-sm">Interval</span>
                            <span className="font-mono text-sm">{strategy.interval || '--'}</span>
                        </div>
                        <div className="flex justify-between py-2">
                            <span className="text-muted-foreground text-sm">Created</span>
                            <span className="font-mono text-sm">
                                {strategy.created_at ? new Date(strategy.created_at).toLocaleDateString() : '--'}
                            </span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
