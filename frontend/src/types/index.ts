// Strategy Types
export interface Strategy {
    id: string
    name: string
    type: string
    symbol: string
    interval: string
    status: 'ACTIVE' | 'PAUSED' | 'STOPPED' | 'ERROR'
    is_active: boolean
    connection_id?: string
    params?: Record<string, unknown>
    created_at: string
    updated_at: string
}

export interface StrategyPerformance {
    strategy_id: string
    realized_pnl: number
    unrealized_pnl: number
    total_trades: number
    win_rate: number
    max_drawdown: number
}

// Order Types
export interface Order {
    ID: string
    Symbol: string
    Side: 'BUY' | 'SELL'
    Type: string
    Price: number
    Qty: number
    Status: string
    StrategyInstanceID?: string
    CreatedAt: string
    UpdatedAt: string
}

// Position Types
export interface Position {
    symbol: string
    side: 'LONG' | 'SHORT'
    quantity: number
    entry_price: number
    mark_price: number
    unrealized_pnl: number
    leverage: number
}

// Balance Types
export interface Balance {
    asset: string
    total: number
    available: number
    margin: number
}

// Connection Types
export interface Connection {
    id: string
    name: string
    exchange: string
    venue: string
    is_active: boolean
    created_at: string
}

// User Types
export interface User {
    id: string
    email: string
}

// API Response Types
export interface ApiResponse<T> {
    data: T
    error?: string
    code?: string
}
