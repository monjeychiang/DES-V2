import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw, Download, Search, Filter } from 'lucide-react'
import { format } from 'date-fns'
import { orderApi } from '@/utils/api'
import { cn, formatCurrency } from '@/utils/helpers'
import type { Order } from '@/types'

function StatusBadge({ status }: { status: string }) {
    const styles: Record<string, string> = {
        NEW: 'bg-blue-100 text-blue-700',
        FILLED: 'bg-green-100 text-green-700',
        PARTIALLY_FILLED: 'bg-yellow-100 text-yellow-700',
        CANCELED: 'bg-gray-100 text-gray-700',
        REJECTED: 'bg-red-100 text-red-700',
    }

    return (
        <span className={cn('px-2 py-1 rounded-full text-xs font-medium', styles[status] || 'bg-gray-100 text-gray-700')}>
            {status}
        </span>
    )
}

export default function OrdersPage() {
    const { t } = useTranslation()
    const [search, setSearch] = useState('')
    const [sideFilter, setSideFilter] = useState<'all' | 'BUY' | 'SELL'>('all')

    const { data: ordersData, isLoading, refetch } = useQuery({
        queryKey: ['orders'],
        queryFn: () => orderApi.getAll(),
        refetchInterval: 5000,
    })

    const orders = (ordersData?.data || []) as Order[]

    const filtered = orders.filter(o => {
        const matchesSearch = !search || o.Symbol.toLowerCase().includes(search.toLowerCase())
        const matchesSide = sideFilter === 'all' || o.Side === sideFilter
        return matchesSearch && matchesSide
    })

    const handleExport = () => {
        const csv = [
            ['ID', 'Symbol', 'Side', 'Type', 'Price', 'Quantity', 'Status', 'Time'].join(','),
            ...filtered.map(o => [
                o.ID,
                o.Symbol,
                o.Side,
                o.Type,
                o.Price,
                o.Qty,
                o.Status,
                o.CreatedAt
            ].join(','))
        ].join('\n')

        const blob = new Blob([csv], { type: 'text/csv' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `orders-${format(new Date(), 'yyyy-MM-dd')}.csv`
        a.click()
        URL.revokeObjectURL(url)
    }

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <h1 className="text-3xl font-bold">{t('order.title')}</h1>
                <div className="flex items-center gap-2">
                    <button
                        onClick={() => refetch()}
                        className="p-2 hover:bg-accent rounded-lg"
                    >
                        <RefreshCw className="w-5 h-5 text-muted-foreground" />
                    </button>
                    <button
                        onClick={handleExport}
                        className="flex items-center gap-2 px-4 py-2 bg-muted text-muted-foreground rounded-lg hover:bg-accent"
                    >
                        <Download className="w-4 h-4" />
                        {t('common.export')}
                    </button>
                </div>
            </div>

            {/* Filters */}
            <div className="flex items-center gap-4">
                <div className="relative flex-1 max-w-md">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <input
                        type="text"
                        placeholder={`${t('common.search')}...`}
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="w-full pl-10 pr-4 py-2 bg-background border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                    />
                </div>
                <div className="flex items-center gap-2">
                    <Filter className="w-4 h-4 text-muted-foreground" />
                    {(['all', 'BUY', 'SELL'] as const).map(f => (
                        <button
                            key={f}
                            onClick={() => setSideFilter(f)}
                            className={cn(
                                'px-3 py-1.5 rounded-lg text-sm font-medium transition-colors',
                                sideFilter === f
                                    ? f === 'BUY' ? 'bg-green-100 text-green-700' : f === 'SELL' ? 'bg-red-100 text-red-700' : 'bg-primary text-primary-foreground'
                                    : 'bg-muted text-muted-foreground hover:bg-accent'
                            )}
                        >
                            {f === 'all' ? 'All' : t(`order.${f.toLowerCase()}`)}
                        </button>
                    ))}
                </div>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-4">
                <div className="bg-card rounded-lg border p-4">
                    <div className="text-sm text-muted-foreground">Total Orders</div>
                    <div className="text-2xl font-bold">{orders.length}</div>
                </div>
                <div className="bg-card rounded-lg border p-4">
                    <div className="text-sm text-muted-foreground">Buy Orders</div>
                    <div className="text-2xl font-bold text-green-600">
                        {orders.filter(o => o.Side === 'BUY').length}
                    </div>
                </div>
                <div className="bg-card rounded-lg border p-4">
                    <div className="text-sm text-muted-foreground">Sell Orders</div>
                    <div className="text-2xl font-bold text-red-600">
                        {orders.filter(o => o.Side === 'SELL').length}
                    </div>
                </div>
            </div>

            {/* Table */}
            <div className="bg-card rounded-xl border overflow-hidden">
                <table className="w-full">
                    <thead>
                        <tr className="border-b bg-muted/50">
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('order.time')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('strategy.symbol')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">{t('order.side')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">Type</th>
                            <th className="px-4 py-3 text-right text-sm font-medium text-muted-foreground">{t('order.price')}</th>
                            <th className="px-4 py-3 text-right text-sm font-medium text-muted-foreground">{t('order.quantity')}</th>
                            <th className="px-4 py-3 text-left text-sm font-medium text-muted-foreground">Status</th>
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
                                    No orders found
                                </td>
                            </tr>
                        ) : (
                            filtered.map(o => (
                                <tr key={o.ID} className="hover:bg-muted/30 transition-colors">
                                    <td className="px-4 py-3 text-sm text-muted-foreground">
                                        {format(new Date(o.CreatedAt), 'yyyy-MM-dd HH:mm:ss')}
                                    </td>
                                    <td className="px-4 py-3 font-mono text-sm">{o.Symbol}</td>
                                    <td className="px-4 py-3">
                                        <span className={cn(
                                            'font-semibold',
                                            o.Side === 'BUY' ? 'text-green-600' : 'text-red-600'
                                        )}>
                                            {o.Side}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-sm">{o.Type}</td>
                                    <td className="px-4 py-3 text-right font-mono">{formatCurrency(o.Price)}</td>
                                    <td className="px-4 py-3 text-right font-mono">{o.Qty}</td>
                                    <td className="px-4 py-3">
                                        <StatusBadge status={o.Status} />
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
