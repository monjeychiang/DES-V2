import React, { useState } from 'react';
import { loginUser, registerUser } from '../api';

const mapAuthError = (mode, code, fallback) => {
    const isLogin = mode === 'login';
    switch (code) {
    case 'MISSING_CREDENTIALS':
        return '請輸入 Email 與密碼。';
    case 'INVALID_EMAIL':
        return 'Email 格式不正確，請重新輸入。';
    case 'EMAIL_ALREADY_REGISTERED':
        return '此 Email 已經註冊，請直接登入或使用其他 Email。';
    case 'INVALID_CREDENTIALS':
        return '帳號或密碼錯誤，請再試一次。';
    case 'INVALID_PAYLOAD':
        return '送出的資料格式有誤，請檢查後再試。';
    case 'INTERNAL_ERROR':
        return isLogin ? '登入暫時失敗，請稍後再試。' : '註冊暫時失敗，請稍後再試。';
    default:
        if (!fallback) {
            return isLogin ? '登入失敗，請稍後再試。' : '註冊失敗，請稍後再試。';
        }
        return fallback;
    }
};

const Login = ({ onLogin }) => {
    const [mode, setMode] = useState('login'); // 'login' | 'register'
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    const handleSubmit = async (e) => {
        e.preventDefault();
        setLoading(true);
        setError('');
        try {
            const fn = mode === 'login' ? loginUser : registerUser;
            const res = await fn(email, password);
            const token = res.data?.token;
            if (token) {
                if (typeof window !== 'undefined') {
                    window.localStorage.setItem('des_token', token);
                }
                onLogin(token, res.data?.user);
            } else {
                setError('伺服器沒有回傳登入憑證，請稍後再試。');
            }
        } catch (err) {
            const code = err.response?.data?.code;
            const fallback = err.response?.data?.error || err.message;
            setError(mapAuthError(mode, code, fallback));
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50">
            <div className="w-full max-w-md bg-white p-8 rounded shadow">
                <h1 className="text-2xl font-bold mb-2 text-gray-800 text-center">DES Trading System</h1>
                <p className="text-sm text-gray-500 mb-6 text-center">
                    {mode === 'login' ? 'Login to your account' : 'Create a new account'}
                </p>

                <div className="flex mb-4">
                    <button
                        className={`flex-1 py-2 text-sm font-semibold border-b-2 ${mode === 'login' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500'}`}
                        onClick={() => setMode('login')}
                        type="button"
                    >
                        Login
                    </button>
                    <button
                        className={`flex-1 py-2 text-sm font-semibold border-b-2 ${mode === 'register' ? 'border-blue-500 text-blue-600' : 'border-transparent text-gray-500'}`}
                        onClick={() => setMode('register')}
                        type="button"
                    >
                        Register
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
                        <input
                            type="email"
                            className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring focus:border-blue-300"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            required
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                        <input
                            type="password"
                            className="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring focus:border-blue-300"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required
                            minLength={6}
                        />
                    </div>
                    {error && <div className="text-sm text-red-500">{error}</div>}
                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full py-2 bg-blue-600 text-white rounded font-semibold text-sm hover:bg-blue-700 disabled:opacity-50"
                    >
                        {loading ? 'Please wait...' : mode === 'login' ? 'Login' : 'Register & Login'}
                    </button>
                </form>
            </div>
        </div>
    );
};

export default Login;
