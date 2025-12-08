import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
    Play,
    Pause,
    Square,
    Zap,
    Plus,
    RefreshCw
} from 'lucide-react'
import { strategyApi, connectionApi } from '@/utils/api'
import { cn } from '@/utils/helpers'
import type { Strategy, Connection } from '@/types'

function StatusBadge({ status, isActive }: { status: string; isActive: boolean }) {
    const { t } = useTranslation()
    const derived = (status || (isActive ? 'ACTIVE' : 'STOPPED')).toUpperCase()

    const styles: Record<string, string> = {
        ACTIVE: 'bg-green-100 text-green-700',
        PAUSED: 'bg-yellow-100 text-yellow-700',
        STOPPED: 'bg-gray-100 text-gray-700',
        ERROR: 'bg-red-100 text-red-700',
    }

    const labels: Record<string, string> = {
        ACTIVE: t('strategy.statusActive'),
        PAUSED: t('strategy.statusPaused'),
        STOPPED: t('strategy.statusStopped'),
        ERROR: t('strategy.statusError'),
    }

    return (
        <span className={cn('px-2 py-1 rounded-full text-xs font-medium', styles[derived] || styles.STOPPED)}>
            {labels[derived] || derived}
        </span>
    )
}

function StrategyActions({
    strategy,
    onAction
}: {
    strategy: Strategy
    onAction: (action: string, id: string) => void
}) {
    const { t } = useTranslation()
    const isActive = strategy.is_active

    return (
        <div className="flex items-center gap-1">
            {isActive ? (
                <>
                    <button
                        onClick={() => onAction('pause', strategy.id)}
                        className="p-1.5 hover:bg-yellow-100 rounded text-yellow-600"
                        title={t('strategy.pause')}
                    >
                        <Pause className="w-4 h-4" />
                    </button>
                    <button
                        onClick={() => onAction('stop', strategy.id)}
                        className="p-1.5 hover:bg-red-100 rounded text-red-600"
                        title={t('strategy.stop')}
                    >
                        <Square className="w-4 h-4" />
                    </button>
                    <button
                        onClick={() => onAction('panic', strategy.id)}
                        className="p-1.5 hover:bg-red-100 rounded text-red-600"
                        title={t('strategy.panicSell')}
                    >
                        <Zap className="w-4 h-4" />
                    </button>
                </>
            ) : (
                <button
                    onClick={() => onAction('start', strategy.id)}
                    disabled={!strategy.connection_id}
                    className="p-1.5 hover:bg-green-100 rounded text-green-600 disabled:opacity-50 disabled:cursor-not-allowed"
                    title={t('strategy.start')}
                >
                    <Play className="w-4 h-4" />
                </button>
            )}
        </div>
    )
}

export default function StrategiesPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [filter, setFilter] = useState<'all' | 'active' | 'stopped'>('all')

    const { data: strategiesData, isLoading, refetch } = useQuery({
        queryKey: ['strategies'],
        queryFn: () => strategyApi.getAll(),
        refetchInterval: 5000,
    })

    const { data: connectionsData } = useQuery({
        queryKey: ['connections'],
        queryFn: () => connectionApi.getAll(),
    })

    const actionMutation = useMutation({
        mutationFn: async ({ action, id }: { action: string; id: string }) => {
            switch (action) {
                case 'start': return strategyApi.start(id)
                case 'pause': return strategyApi.pause(id)
                case 'stop': return strategyApi.stop(id)
                case 'panic': return strategyApi.panicSell(id)
                default: throw new Error('Unknown action')
            }
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['strategies'] })
        },
    })

    const bindMutation = useMutation({
        mutationFn: ({ strategyId, connectionId }: { strategyId: string; connectionId: string }) =>
            strategyApi.bindConnection(strategyId, connectionId),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['strategies'] })
        },
    })

    const handleAction = (action: string, id: string) => {
        if (action === 'panic') {
            if (!confirm(t('strategy.panicSell') + '?')) return
        }
        actionMutation.mutate({ action, id })
    }

    const strategies = (strategiesData?.data || []) as Strategy[]
    const connections = (connectionsData?.data || []) as Connection[]

    const filtered = strategies.filter(s => {
        if (filter === 'active') return s.is_active
        if (filter === 'stopped') return !s.is_active
        return true
    })

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <h1 className="text-3xl font-bold">{t('strategy.title')}</h1>
                <div className="flex items-center gap-2">
                    <button
                        onClick={() => refetch()}
                        className="p-2 hover:bg-accent rounded-lg"
                    >
                        <RefreshCw className="w-5 h-5 text-muted-foreground" />
                    </button>
                    <button className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90">
                        <Plus className="w-4 h-4" />
                        {t('common.create')}
                    </button>
                </div>
            </div>

            {/* Filters */}
            <div className="flex gap-2">
                {(['all', 'active', 'stopped'] as const).map(f => (
                    <button
                        key={f}
                        onClick={() => setFilter(f)}
                        className={cn(
                            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                            filter === f
                                ? 'bg-primary text-primary-foreground'
                                : 'bg-muted text-muted-foreground hover:bg-accent'
                        )}
                    >
                        {f === 'all' ? 'All' : f === 'active' ? t('strategy.statusActive') : t('strategy.statusStopped')}
                    </button>
                ))}
            </div>

            {/* Table */}
            <div className="bg-card rounded-xl border overflow-hidden">
                <table className="w-full">
                    <thead>
                        <tr className="border-b bg-muted/50">
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.name')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.type')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.symbol')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.connection')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.status')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.actions')}</th>
                            <th className="px-4 py-3"></th>
                        </tr>
                    </thead>
                    <tbody className="divide-y">
                        {isLoading ? (
                            <tr>
                                <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                                    {t('common.loading')}
                                </td>
                            </tr>
                        ) : filtered.length === 0 ? (
                            <tr>
                                <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                                    No strategies found
                                </td>
                            </tr>
                        ) : (
                            filtered.map(s => (
                                <tr key={s.id} className="hover:bg-muted/30 transition-colors">
                                    <td className="px-4 py-3 font-medium">{s.name}</td>
                                    <td className="px-4 py-3">
                                        <span className="px-2 py-1 bg-blue-100 text-blue-700 text-xs rounded-full font-medium">
                                            {s.type}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 font-mono text-sm">{s.symbol}</td>
                                    <td className="px-4 py-3">
                                        <select
                                            className="text-sm border rounded px-2 py-1 bg-background"
                                            value={s.connection_id || ''}
                                            onChange={(e) => bindMutation.mutate({ strategyId: s.id, connectionId: e.target.value })}
                                        >
                                            <option value="">Not bound</option>
                                            {connections.map(c => (
                                                <option key={c.id} value={c.id}>{c.name}</option>
                                            ))}
                                        </select>
                                    </td>
                                    <td className="px-4 py-3">
                                        <StatusBadge status={s.status} isActive={s.is_active} />
                                    </td>
                                    <td className="px-4 py-3">
                                        <StrategyActions strategy={s} onAction={handleAction} />
                                    </td>
                                    <td className="px-4 py-3">
                                        <Link
                                            to={`/strategies/${s.id}`}
                                            className="text-sm text-primary hover:underline"
                                        >
                                            {t('strategy.details')} →
                                        </Link>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
