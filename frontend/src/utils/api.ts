import axios from 'axios'
import { useAuthStore } from '@/stores/authStore'

const api = axios.create({
    baseURL: '/api',
    timeout: 10000,
    headers: {
        'Content-Type': 'application/json',
    },
})

// Request interceptor - add auth token
api.interceptors.request.use(
    (config) => {
        const token = useAuthStore.getState().token
        if (token) {
            config.headers.Authorization = `Bearer ${token}`
        }
        return config
    },
    (error) => Promise.reject(error)
)

// Response interceptor - handle auth errors
api.interceptors.response.use(
    (response) => response,
    (error) => {
        if (error.response?.status === 401) {
            useAuthStore.getState().logout()
        }
        return Promise.reject(error)
    }
)

export default api

// Auth API
export const authApi = {
    login: (email: string, password: string) =>
        api.post('/auth/login', { email, password }),
    register: (email: string, password: string) =>
        api.post('/auth/register', { email, password }),
}

// Strategy API
export const strategyApi = {
    getAll: () => api.get('/strategies'),
    getById: (id: string) => api.get(`/strategies/${id}`),
    start: (id: string) => api.post(`/strategies/${id}/start`),
    pause: (id: string) => api.post(`/strategies/${id}/pause`),
    stop: (id: string) => api.post(`/strategies/${id}/stop`),
    panicSell: (id: string) => api.post(`/strategies/${id}/panic-sell`),
    getPerformance: (id: string) => api.get(`/strategies/${id}/performance`),
    bindConnection: (id: string, connectionId: string) =>
        api.post(`/strategies/${id}/bind`, { connection_id: connectionId }),
}

// Order API
export const orderApi = {
    getAll: () => api.get('/orders'),
}

// Balance API
export const balanceApi = {
    get: () => api.get('/balance'),
}

// Position API
export const positionApi = {
    getAll: () => api.get('/positions'),
}

// Connection API
export const connectionApi = {
    getAll: () => api.get('/connections'),
    create: (data: { name: string; api_key: string; api_secret: string; exchange: string; venue: string }) =>
        api.post('/connections', data),
    delete: (id: string) => api.delete(`/connections/${id}`),
    test: (id: string) => api.post(`/connections/${id}/test`),
}

// System API
export const systemApi = {
    getStatus: () => api.get('/system/status'),
}
