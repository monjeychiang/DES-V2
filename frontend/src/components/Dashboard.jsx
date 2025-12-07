import React from 'react';
import BalanceWidget from './BalanceWidget';
import PositionCard from './PositionCard';
import StrategyList from './StrategyList';
import OrderTable from './OrderTable';
import SystemStatusBar from './SystemStatusBar';
import ConnectionsPanel from './ConnectionsPanel';

const Dashboard = ({ onLogout }) => {
    return (
        <div className="min-h-screen bg-gray-50 p-8">
            <header className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-gray-800">DES Trading System</h1>
                        <p className="text-gray-500">Real-time Strategy Monitor</p>
                    </div>
                    {onLogout && (
                        <button
                            type="button"
                            onClick={onLogout}
                            className="px-3 py-2 text-xs font-semibold text-gray-600 bg-white border border-gray-300 rounded hover:bg-gray-100"
                        >
                            Logout
                        </button>
                    )}
                </div>
            </header>

            <SystemStatusBar />

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                {/* Left Column: Balance & Positions */}
                <div className="lg:col-span-1">
                    <BalanceWidget />
                    <PositionCard />
                    <ConnectionsPanel />
                </div>

                {/* Right Column: Strategies & Orders */}
                <div className="lg:col-span-2 space-y-8">
                    <StrategyList />
                    <OrderTable />
                </div>
            </div>
        </div>
    );
};

export default Dashboard;
