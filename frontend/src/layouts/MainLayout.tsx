import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import Header from './Header'
import StatusBar from './StatusBar'

export default function MainLayout() {
    return (
        <div className="min-h-screen flex bg-background">
            {/* Sidebar */}
            <Sidebar />

            {/* Main Content Area */}
            <div className="flex-1 flex flex-col lg:ml-64">
                {/* Header */}
                <Header />

                {/* Page Content */}
                <main className="flex-1 p-4 lg:p-6 overflow-auto">
                    <Outlet />
                </main>

                {/* Status Bar */}
                <StatusBar />
            </div>
        </div>
    )
}
