import React, { useEffect, useState } from 'react';
import api from '../api';

const PositionCard = () => {
    const [positions, setPositions] = useState([]);

    useEffect(() => {
        const fetchPositions = async () => {
            try {
                const response = await api.get('/positions');
                setPositions(response.data || []);
            } catch (error) {
                console.error('Failed to fetch positions:', error);
            }
        };
        fetchPositions();
        const interval = setInterval(fetchPositions, 3000);
        return () => clearInterval(interval);
    }, []);

    return (
        <div className="bg-white p-4 rounded-lg shadow">
            <h2 className="text-xl font-bold mb-4">Positions</h2>
            <div className="grid grid-cols-1 gap-4">
                {positions.map((p) => (
                    <div key={p.Symbol} className="border p-3 rounded flex justify-between items-center">
                        <div>
                            <div className="font-bold text-lg">{p.Symbol}</div>
                            <div className="text-gray-500 text-sm">Avg: {Number(p.AvgPrice ?? 0).toFixed(2)}</div>
                        </div>
                        <div className="text-right">
                            <div className={`text-xl font-bold ${p.Qty > 0 ? 'text-green-600' : 'text-red-600'}`}>
                                {Number(p.Qty ?? 0)}
                            </div>
                            <div className="text-xs text-gray-400">Qty</div>
                        </div>
                    </div>
                ))}
                {positions.length === 0 && (
                    <div className="text-center text-gray-500 py-4">No open positions</div>
                )}
            </div>
        </div>
    );
};

export default PositionCard;
