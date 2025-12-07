import React, { useEffect, useState } from 'react';
import { getConnections, createConnection, deactivateConnection } from '../api';

const ConnectionsPanel = () => {
    const [connections, setConnections] = useState([]);
    const [form, setForm] = useState({
        name: '',
        exchange_type: 'binance-spot',
        api_key: '',
        api_secret: '',
    });
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const loadConnections = async () => {
        try {
            const res = await getConnections();
            setConnections(res.data || []);
        } catch (err) {
            setError(err.response?.data?.error || err.message);
        }
    };

    useEffect(() => {
        loadConnections();
    }, []);

    const handleChange = (e) => {
        const { name, value } = e.target;
        setForm((prev) => ({ ...prev, [name]: value }));
    };

    const handleCreate = async (e) => {
        e.preventDefault();
        setLoading(true);
        setError('');
        try {
            await createConnection(form);
            setForm({
                name: '',
                exchange_type: 'binance-spot',
                api_key: '',
                api_secret: '',
            });
            await loadConnections();
        } catch (err) {
            setError(err.response?.data?.error || err.message);
        } finally {
            setLoading(false);
        }
    };

    const handleDeactivate = async (id) => {
        if (!window.confirm('Deactivate this connection?')) return;
        try {
            await deactivateConnection(id);
            await loadConnections();
        } catch (err) {
            setError(err.response?.data?.error || err.message);
        }
    };

    return (
        <div className="bg-white p-4 rounded-lg shadow mt-6">
            <h2 className="text-xl font-bold mb-3">Exchange Connections</h2>

            <form className="space-y-2 mb-4" onSubmit={handleCreate}>
                <div className="flex gap-2">
                    <input
                        type="text"
                        name="name"
                        placeholder="Name (e.g., Binance Spot)"
                        className="flex-1 border rounded px-2 py-1 text-sm"
                        value={form.name}
                        onChange={handleChange}
                        required
                    />
                    <select
                        name="exchange_type"
                        className="border rounded px-2 py-1 text-sm"
                        value={form.exchange_type}
                        onChange={handleChange}
                    >
                        <option value="binance-spot">Binance Spot</option>
                        <option value="binance-usdtfut">Binance USDT Futures</option>
                        <option value="binance-coinfut">Binance Coin Futures</option>
                    </select>
                </div>
                <input
                    type="text"
                    name="api_key"
                    placeholder="API Key"
                    className="w-full border rounded px-2 py-1 text-sm"
                    value={form.api_key}
                    onChange={handleChange}
                    required
                />
                <input
                    type="password"
                    name="api_secret"
                    placeholder="API Secret"
                    className="w-full border rounded px-2 py-1 text-sm"
                    value={form.api_secret}
                    onChange={handleChange}
                    required
                />
                {error && <div className="text-xs text-red-500">{error}</div>}
                <button
                    type="submit"
                    disabled={loading}
                    className="px-3 py-1 bg-blue-600 text-white rounded text-xs font-semibold hover:bg-blue-700 disabled:opacity-50"
                >
                    {loading ? 'Saving...' : 'Add Connection'}
                </button>
            </form>

            <div className="space-y-2">
                {connections.map((c) => (
                    <div key={c.id} className="border rounded px-3 py-2 flex items-center justify-between">
                        <div>
                            <div className="text-sm font-semibold">{c.name}</div>
                            <div className="text-xs text-gray-500">{c.exchange_type}</div>
                        </div>
                        <div className="flex items-center gap-2">
                            <span
                                className={`px-2 py-0.5 rounded text-xs ${
                                    c.is_active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                                }`}
                            >
                                {c.is_active ? 'Active' : 'Inactive'}
                            </span>
                            {c.is_active && (
                                <button
                                    type="button"
                                    onClick={() => handleDeactivate(c.id)}
                                    className="text-xs text-red-600 hover:underline"
                                >
                                    Deactivate
                                </button>
                            )}
                        </div>
                    </div>
                ))}
                {connections.length === 0 && (
                    <div className="text-xs text-gray-500">No connections configured yet.</div>
                )}
            </div>
        </div>
    );
};

export default ConnectionsPanel;

