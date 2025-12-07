import React, { useEffect, useState } from 'react';
import { getStrategyPerformance } from '../api';

const PerformanceModal = ({ strategy, onClose }) => {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [data, setData] = useState({ daily: [], total_pnl: 0 });

    useEffect(() => {
        if (!strategy) return;
        const run = async () => {
            setLoading(true);
            try {
                const res = await getStrategyPerformance(strategy.id);
                setData(res.data || { daily: [], total_pnl: 0 });
                setError('');
            } catch (err) {
                setError(err.response?.data?.error || err.message);
            } finally {
                setLoading(false);
            }
        };
        run();
    }, [strategy]);

    const maxAbs = Math.max(...(data.daily || []).map((p) => Math.abs(p.PNL || p.pnl || 0)), 0.01);

    return (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-white w-full max-w-3xl rounded shadow-lg p-6">
                <div className="flex items-center justify-between mb-4">
                    <div>
                        <h3 className="text-lg font-bold">Performance - {strategy?.name}</h3>
                        <p className="text-xs text-gray-500">
                            累積 PnL: {Number(data.total_pnl || 0).toFixed(2)}
                        </p>
                    </div>
                    <button
                        type="button"
                        onClick={onClose}
                        className="text-gray-500 hover:text-gray-700 text-sm"
                    >
                        Close
                    </button>
                </div>
                {loading && <p className="text-sm text-gray-500">載入中...</p>}
                {error && <p className="text-sm text-red-500">{error}</p>}
                {!loading && !error && (
                    <div className="space-y-4">
                        <div>
                            <h4 className="text-sm font-semibold mb-2">每日盈虧</h4>
                            <div className="space-y-1 max-h-64 overflow-y-auto pr-1">
                                {(data.daily || []).map((p) => {
                                    const pnl = p.PNL ?? p.pnl ?? 0;
                                    const barWidth = Math.min(Math.abs(pnl) / maxAbs, 1) * 100;
                                    const barColor = pnl >= 0 ? 'bg-green-500' : 'bg-red-500';
                                    return (
                                        <div key={p.date} className="flex items-center gap-2 text-xs">
                                            <span className="w-24 text-gray-600">{p.date}</span>
                                            <div className="flex-1 h-3 bg-gray-100 rounded">
                                                <div
                                                    className={`h-3 ${barColor} rounded`}
                                                    style={{ width: `${barWidth}%` }}
                                                />
                                            </div>
                                            <span className="w-16 text-right font-mono">
                                                {pnl.toFixed(2)}
                                            </span>
                                        </div>
                                    );
                                })}
                                {(data.daily || []).length === 0 && (
                                    <div className="text-xs text-gray-500">尚無交易資料</div>
                                )}
                            </div>
                        </div>
                        <div>
                            <h4 className="text-sm font-semibold mb-2">累積 PnL (Equity)</h4>
                            <div className="space-y-1 max-h-64 overflow-y-auto pr-1">
                                {(data.daily || []).map((p) => {
                                    const equity = p.Equity ?? p.equity ?? 0;
                                    return (
                                        <div key={p.date} className="flex items-center gap-2 text-xs">
                                            <span className="w-24 text-gray-600">{p.date}</span>
                                            <span className="text-right font-mono">{equity.toFixed(2)}</span>
                                        </div>
                                    );
                                })}
                                {(data.daily || []).length === 0 && (
                                    <div className="text-xs text-gray-500">尚無累積數據</div>
                                )}
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default PerformanceModal;
