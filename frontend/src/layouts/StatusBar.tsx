import { useEffect, useState } from 'react'
import { systemApi } from '@/utils/api'

interface SystemStatus {
    mode: string
    venue: string
    version: string
    server_time: string
}

export default function StatusBar() {
    const [status, setStatus] = useState<SystemStatus | null>(null)

    useEffect(() => {
        const fetchStatus = async () => {
            try {
                const res = await systemApi.getStatus()
                setStatus(res.data)
            } catch (err) {
                console.error('Failed to fetch system status', err)
            }
        }

        fetchStatus()
        const interval = setInterval(fetchStatus, 5000)
        return () => clearInterval(interval)
    }, [])

    const modeColor = status?.mode === 'LIVE' ? 'text-green-500' : 'text-yellow-500'

    return (
        <footer className="h-8 border-t bg-card flex items-center justify-between px-6 text-xs text-muted-foreground">
            <div className="flex items-center gap-4">
                <span>
                    Mode: <span className={`font-semibold ${modeColor}`}>{status?.mode || '--'}</span>
                </span>
                <span>Venue: {status?.venue || '--'}</span>
            </div>
            <div className="flex items-center gap-4">
                <span>Server: {status?.server_time || '--'}</span>
                <span>v{status?.version || '2.0.0'}</span>
            </div>
        </footer>
    )
}
