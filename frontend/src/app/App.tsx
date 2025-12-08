import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/authStore'
import { useThemeEffect } from '@/hooks/useThemeEffect'
import MainLayout from '@/layouts/MainLayout'
import AuthLayout from '@/layouts/AuthLayout'
import LoginPage from '@/pages/LoginPage'
import DashboardPage from '@/pages/DashboardPage'
import StrategiesPage from '@/pages/StrategiesPage'
import StrategyDetailPage from '@/pages/StrategyDetailPage'
import OrdersPage from '@/pages/OrdersPage'
import PerformancePage from '@/pages/PerformancePage'
import SettingsPage from '@/pages/SettingsPage'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
    const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
    if (!isAuthenticated) {
        return <Navigate to="/login" replace />
    }
    return <>{children}</>
}

function App() {
    // Apply theme class to document
    useThemeEffect()

    return (
        <BrowserRouter>
            <Routes>
                {/* Auth Routes */}
                <Route element={<AuthLayout />}>
                    <Route path="/login" element={<LoginPage />} />
                </Route>

                {/* Protected Routes */}
                <Route
                    element={
                        <ProtectedRoute>
                            <MainLayout />
                        </ProtectedRoute>
                    }
                >
                    <Route path="/" element={<DashboardPage />} />
                    <Route path="/strategies" element={<StrategiesPage />} />
                    <Route path="/strategies/:id" element={<StrategyDetailPage />} />
                    <Route path="/orders" element={<OrdersPage />} />
                    <Route path="/performance" element={<PerformancePage />} />
                    <Route path="/settings" element={<SettingsPage />} />
                </Route>

                {/* Fallback */}
                <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
        </BrowserRouter>
    )
}

export default App
