import React, { useEffect, useState } from 'react';
import api from '../api';

const OrderTable = () => {
    const [orders, setOrders] = useState([]);

    useEffect(() => {
        const fetchOrders = async () => {
            try {
                const response = await api.get('/orders');
                setOrders(response.data || []);
            } catch (error) {
                console.error('Failed to fetch orders:', error);
            }
        };
        fetchOrders();
        const interval = setInterval(fetchOrders, 3000);
        return () => clearInterval(interval);
    }, []);

    return (
        <div className="bg-white p-4 rounded-lg shadow mt-4">
            <h2 className="text-xl font-bold mb-4">Recent Orders</h2>
            <div className="overflow-x-auto">
                <table className="min-w-full table-auto">
                    <thead>
                        <tr className="bg-gray-100">
                            <th className="px-4 py-2 text-left">Time</th>
                            <th className="px-4 py-2 text-left">Symbol</th>
                            <th className="px-4 py-2 text-left">Side</th>
                            <th className="px-4 py-2 text-left">Price</th>
                            <th className="px-4 py-2 text-left">Qty</th>
                            <th className="px-4 py-2 text-left">Status</th>
                        </tr>
                    </thead>
                    <tbody>
                        {orders.map((o) => (
                            <tr key={o.ID} className="border-b">
                                <td className="px-4 py-2 text-sm">{new Date(o.CreatedAt).toLocaleTimeString()}</td>
                                <td className="px-4 py-2">{o.Symbol}</td>
                                <td className={`px-4 py-2 font-bold ${o.Side === 'BUY' ? 'text-green-600' : 'text-red-600'}`}>
                                    {o.Side}
                                </td>
                                <td className="px-4 py-2">{o.Price.toFixed(2)}</td>
                                <td className="px-4 py-2">{o.Qty}</td>
                                <td className="px-4 py-2 text-sm">{o.Status}</td>
                            </tr>
                        ))}
                        {orders.length === 0 && (
                            <tr>
                                <td colSpan="6" className="px-4 py-2 text-center text-gray-500">No open orders</td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

export default OrderTable;
