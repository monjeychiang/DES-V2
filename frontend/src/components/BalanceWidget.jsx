import React, { useEffect, useState } from 'react';
import api from '../api';

const BalanceWidget = () => {
    const [balance, setBalance] = useState({ Total: 0, Available: 0, Locked: 0 });

    useEffect(() => {
        const fetchBalance = async () => {
            try {
                const response = await api.get('/balance');
                if (response.data) {
                    setBalance(response.data);
                }
            } catch (error) {
                console.error('Failed to fetch balance:', error);
            }
        };
        fetchBalance();
        const interval = setInterval(fetchBalance, 5000);
        return () => clearInterval(interval);
    }, []);

    const total = Number(balance.Total ?? 0);
    const available = Number(balance.Available ?? 0);
    const locked = Number(balance.Locked ?? 0);

    return (
        <div className="bg-blue-600 text-white p-6 rounded-lg shadow mb-6">
            <h2 className="text-sm uppercase opacity-80 mb-1">Total Balance</h2>
            <div className="text-4xl font-bold mb-4">${total.toFixed(2)}</div>
            <div className="flex space-x-8">
                <div>
                    <div className="text-xs opacity-80">Available</div>
                    <div className="text-lg font-semibold">${available.toFixed(2)}</div>
                </div>
                <div>
                    <div className="text-xs opacity-80">Locked</div>
                    <div className="text-lg font-semibold">${locked.toFixed(2)}</div>
                </div>
            </div>
        </div>
    );
};

export default BalanceWidget;
