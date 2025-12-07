import React, { useEffect, useState } from 'react';
import { getSystemStatus } from '../api';

const SystemStatusBar = () => {
    const [status, setStatus] = useState(null);
    const [error, setError] = useState('');

    const fetchStatus = async () => {
        try {
            const res = await getSystemStatus();
            setStatus(res.data);
            setError('');
        } catch (err) {
            setError(err.response?.data?.error || err.message);
        }
    };

    useEffect(() => {
        fetchStatus();
        const interval = setInterval(fetchStatus, 5000);
        return () => clearInterval(interval);
    }, []);

    const modeBadgeClass =
        status?.mode === 'DRY_RUN'
            ? 'bg-yellow-100 text-yellow-800 border border-yellow-200'
            : 'bg-emerald-100 text-emerald-800 border border-emerald-200';

    const serverTime = status?.server_time ? new Date(status.server_time).toLocaleString() : '--';
    const symbols = status?.symbols?.length ? status.symbols.join(', ') : 'N/A';

    return (
        <div className="bg-white p-4 rounded shadow mb-6 space-y-2">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-3">
                    <span className={`px-3 py-1 rounded-full text-xs font-semibold ${modeBadgeClass}`}>
                        {status?.mode || '...'}
                    </span>
                    <span className="text-gray-700 font-semibold">
                        Venue: <span className="font-mono text-sm">{status?.venue || 'N/A'}</span>
                    </span>
                </div>
                <div className="text-sm text-gray-500">Server: {serverTime}</div>
            </div>
            <div className="flex flex-wrap gap-3 text-sm text-gray-600">
                <span className="px-2 py-1 bg-gray-100 rounded border border-gray-200">
                    Symbols: {symbols}
                </span>
                <span className="px-2 py-1 bg-gray-100 rounded border border-gray-200">
                    Feed: {status?.use_mock_feed ? 'Mock' : 'Live'}
                </span>
                <span className="px-2 py-1 bg-gray-100 rounded border border-gray-200">
                    Version: {status?.version || 'dev'}
                </span>
            </div>
            {error && <div className="text-xs text-red-500">Failed to load system status: {error}</div>}
        </div>
    );
};

export default SystemStatusBar;
