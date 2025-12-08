import { Outlet, Navigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/authStore'
import { Activity } from 'lucide-react'

export default function AuthLayout() {
    const isAuthenticated = useAuthStore((state) => state.isAuthenticated)

    if (isAuthenticated) {
        return <Navigate to="/" replace />
    }

    return (
        <div className="min-h-screen flex bg-slate-900 text-white overflow-hidden relative">
            {/* Background */}
            <div className="absolute inset-0 overflow-hidden">
                <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-blue-600/20 rounded-full blur-[100px]" />
                <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-purple-600/20 rounded-full blur-[100px]" />
            </div>

            {/* Left Side - Brand */}
            <div className="hidden lg:flex lg:w-1/2 flex-col justify-center items-center relative z-10 p-12">
                <div className="max-w-md">
                    <div className="flex items-center gap-3 mb-8">
                        <div className="w-12 h-12 bg-blue-600 rounded-xl flex items-center justify-center shadow-lg shadow-blue-600/30">
                            <Activity className="w-8 h-8 text-white" />
                        </div>
                        <h1 className="text-4xl font-bold tracking-tight">DES Trading</h1>
                    </div>
                    <h2 className="text-3xl font-bold mb-6 text-gray-100 leading-tight">
                        專業級量化交易系統
                        <span className="block text-blue-400 mt-2">智能、快速、穩定</span>
                    </h2>
                </div>
            </div>

            {/* Right Side - Auth Form */}
            <div className="w-full lg:w-1/2 flex items-center justify-center relative z-10 p-6">
                <div className="w-full max-w-md bg-slate-800/50 backdrop-blur-xl border border-slate-700/50 rounded-2xl p-8 shadow-2xl">
                    <Outlet />
                </div>
            </div>
        </div>
    )
}
