import React, { useEffect, useState } from 'react';
import {
    getStrategies,
    startStrategy,
    pauseStrategy,
    stopStrategy,
    panicSellStrategy,
    getConnections,
    bindStrategyConnection,
    getSystemStatus,
} from '../api';
import EditStrategyModal from './EditStrategyModal';
import PerformanceModal from './PerformanceModal';

function StatusBadge({ status, isActive }) {
    const derived = (status || (isActive ? 'ACTIVE' : 'INACTIVE')).toUpperCase();
    const colorClass = (() => {
        switch (derived) {
        case 'ACTIVE':
            return 'bg-green-100 text-green-800';
        case 'PAUSED':
            return 'bg-yellow-100 text-yellow-800';
        case 'STOPPED':
            return 'bg-gray-200 text-gray-700';
        default:
            return 'bg-red-100 text-red-800';
        }
    })();

    return (
        <span className={`px-2 py-1 rounded text-xs font-semibold ${colorClass}`}>
            {derived}
        </span>
    );
}

function StrategyActions({ status, isActive, isDryRun, isBound, onAction }) {
    const derived = (status || (isActive ? 'ACTIVE' : 'INACTIVE')).toUpperCase();
    const canStart = derived !== 'ACTIVE';
    const canControl = derived === 'ACTIVE';

    const startDisabled = !isDryRun && !isBound;
    const startTitle = !isDryRun && !isBound ? '請先綁定交易連線才能啟用實盤策略' : '';

    return (
        <>
            {canControl && (
                <>
                    <button
                        type="button"
                        onClick={() => onAction('pause')}
                        className="bg-yellow-500 text-white px-2 py-1 rounded text-xs hover:bg-yellow-600"
                    >
                        Pause
                    </button>
                    <button
                        type="button"
                        onClick={() => onAction('stop')}
                        className="bg-red-500 text-white px-2 py-1 rounded text-xs hover:bg-red-600"
                    >
                        Stop
                    </button>
                    <button
                        type="button"
                        onClick={() => onAction('panic')}
                        className="bg-red-700 text-white px-2 py-1 rounded text-xs hover:bg-red-800 font-bold"
                    >
                        PANIC
                    </button>
                </>
            )}
            {canStart && (
                <button
                    type="button"
                    onClick={() => onAction('start')}
                    disabled={startDisabled}
                    title={startTitle}
                    className={`px-2 py-1 rounded text-xs ${
                        startDisabled
                            ? 'bg-gray-300 text-gray-500 cursor-not-allowed'
                            : 'bg-green-500 text-white hover:bg-green-600'
                    }`}
                >
                    Start
                </button>
            )}
        </>
    );
}

function ConnectionSelector({ strategy, connections, onBind }) {
    const currentId = strategy.connection_id || '';

    const handleChange = async (e) => {
        const value = e.target.value;
        await onBind(value);
    };

    if (!connections.length) {
        return <span className="text-xs text-gray-400">No connections</span>;
    }

    return (
        <select
            className="border rounded px-2 py-1 text-xs"
            value={currentId}
            onChange={handleChange}
        >
            <option value="">Not bound</option>
            {connections.map((c) => (
                <option key={c.id} value={c.id}>
                    {c.name}
                </option>
            ))}
        </select>
    );
}

const StrategyList = () => {
    const [strategies, setStrategies] = useState([]);
    const [editingStrategy, setEditingStrategy] = useState(null);
    const [connections, setConnections] = useState([]);
    const [error, setError] = useState('');
    const [isDryRun, setIsDryRun] = useState(false);
    const [perfStrategy, setPerfStrategy] = useState(null);

    const fetchStrategies = async () => {
        try {
            const response = await getStrategies();
            setStrategies(response.data || []);
        } catch (err) {
            console.error('Failed to fetch strategies', err);
        }
    };

    const fetchConnections = async () => {
        try {
            const res = await getConnections();
            setConnections(res.data || []);
        } catch (err) {
            console.error('Failed to fetch connections', err);
            setError('無法載入連線資訊，請稍後再試。');
        }
    };

    const fetchSystemMode = async () => {
        try {
            const res = await getSystemStatus();
            setIsDryRun(res.data?.mode === 'DRY_RUN');
        } catch (err) {
            console.error('Failed to fetch system status', err);
        }
    };

    useEffect(() => {
        // Initial load
        (async () => {
            await Promise.all([fetchStrategies(), fetchConnections(), fetchSystemMode()]);
        })();
        const interval = setInterval(fetchStrategies, 5000);
        return () => clearInterval(interval);
    }, []);

    const handleAction = async (action, id) => {
        try {
            if (action === 'start') await startStrategy(id);
            if (action === 'pause') await pauseStrategy(id);
            if (action === 'stop') await stopStrategy(id);
            if (action === 'panic') {
                if (
                    window.confirm(
                        'Are you sure you want to PANIC SELL? This will close all positions immediately.',
                    )
                ) {
                    await panicSellStrategy(id);
                }
            }
            fetchStrategies(); // Refresh list
        } catch (err) {
            alert(`Action failed: ${err.response?.data?.error || err.message}`);
        }
    };

    return (
        <div className="bg-white p-4 rounded shadow">
            <h2 className="text-xl font-bold mb-4">Active Strategies</h2>
            {error && <p className="text-xs text-red-500 mb-2">{error}</p>}
            <div className="overflow-x-auto">
                <table className="min-w-full table-auto">
                    <thead>
                        <tr className="bg-gray-100">
                            <th className="px-4 py-2 text-left">Name</th>
                            <th className="px-4 py-2 text-left">Type</th>
                            <th className="px-4 py-2 text-left">Symbol</th>
                            <th className="px-4 py-2 text-left">Connection</th>
                            <th className="px-4 py-2 text-left">Status</th>
                            <th className="px-4 py-2 text-left">Actions</th>
                            <th className="px-4 py-2 text-left">Performance</th>
                        </tr>
                    </thead>
                    <tbody>
                        {strategies.map((s) => {
                            const isBound = !!s.connection_id;
                            return (
                                <tr key={s.id} className="border-b">
                                    <td className="px-4 py-2">{s.name}</td>
                                    <td className="px-4 py-2 uppercase text-xs font-semibold text-gray-700">
                                        {s.type}
                                    </td>
                                    <td className="px-4 py-2 font-mono text-sm">{s.symbol}</td>
                                    <td className="px-4 py-2">
                                        <ConnectionSelector
                                            strategy={s}
                                            connections={connections}
                                            onBind={async (connectionId) => {
                                                try {
                                                    await bindStrategyConnection(s.id, connectionId);
                                                    fetchStrategies();
                                                } catch (err) {
                                                    const msg =
                                                        err.response?.data?.error || err.message;
                                                    alert(`Failed to bind connection: ${msg}`);
                                                }
                                            }}
                                        />
                                    </td>
                                    <td className="px-4 py-2">
                                        <StatusBadge status={s.status} isActive={s.is_active} />
                                    </td>
                                    <td className="px-4 py-2 space-x-2 whitespace-nowrap">
                                        <StrategyActions
                                            status={s.status}
                                            isActive={s.is_active}
                                            isDryRun={isDryRun}
                                            isBound={isBound}
                                            onAction={(action) => handleAction(action, s.id)}
                                        />
                                        <button
                                            type="button"
                                            onClick={() => setEditingStrategy(s)}
                                            className="bg-blue-500 text-white px-2 py-1 rounded text-xs hover:bg-blue-600"
                                        >
                                            Edit
                                        </button>
                                    </td>
                                    <td className="px-4 py-2">
                                        <button
                                            type="button"
                                            onClick={() => setPerfStrategy(s)}
                                            className="text-xs text-blue-600 hover:underline"
                                        >
                                            View
                                        </button>
                                    </td>
                                </tr>
                            );
                        })}
                    </tbody>
                </table>
            </div>
            {editingStrategy && (
                <EditStrategyModal
                    strategy={editingStrategy}
                    onClose={() => setEditingStrategy(null)}
                    onUpdate={fetchStrategies}
                />
            )}
            {perfStrategy && (
                <PerformanceModal
                    strategy={perfStrategy}
                    onClose={() => setPerfStrategy(null)}
                />
            )}
        </div>
    );
};

export default StrategyList;
