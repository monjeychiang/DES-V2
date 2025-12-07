import axios from 'axios';

const api = axios.create({
    baseURL: 'http://localhost:8080/api',
});

api.interceptors.request.use((config) => {
    if (typeof window !== 'undefined') {
        const token = window.localStorage.getItem('des_token');
        if (token) {
            config.headers = config.headers || {};
            config.headers.Authorization = `Bearer ${token}`;
        }
    }
    return config;
});

// Auth
export const registerUser = (email, password) => api.post('/auth/register', { email, password });
export const loginUser = (email, password) => api.post('/auth/login', { email, password });

export const getStrategies = () => api.get('/strategies');
export const getSystemStatus = () => api.get('/system/status');
export const getOrders = () => api.get('/orders');
export const getPositions = () => api.get('/positions');
export const getBalance = () => api.get('/balance');
export const getRiskMetrics = () => api.get('/risk');

// Strategy Controls
export const startStrategy = (id) => api.post(`/strategies/${id}/start`);
export const pauseStrategy = (id) => api.post(`/strategies/${id}/pause`);
export const stopStrategy = (id) => api.post(`/strategies/${id}/stop`);
export const panicSellStrategy = (id) => api.post(`/strategies/${id}/panic`);
export const updateStrategyParams = (id, params) => api.put(`/strategies/${id}/params`, params);
export const getStrategyPerformance = (id, query = {}) => api.get(`/strategies/${id}/performance`, { params: query });

// Connections
export const getConnections = () => api.get('/connections');
export const createConnection = (payload) => api.post('/connections', payload);
export const deactivateConnection = (id) => api.delete(`/connections/${id}`);
export const bindStrategyConnection = (id, connectionId) =>
    api.put(`/strategies/${id}/binding`, { connection_id: connectionId });

export default api;
