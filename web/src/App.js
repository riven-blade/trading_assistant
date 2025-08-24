import React, { useState, useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { message } from 'antd';
import MainLayout from './components/Layout/MainLayout';
import Login from './pages/Login';
import Positions from './pages/Positions';
import Orders from './pages/Orders';
import TradingPairs from './pages/TradingPairs';
import Balances from './pages/Balances';
import ChartPage from './pages/ChartPage';
import { getToken, setToken, removeToken } from './utils/auth';
import api from './services/api';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (token) {
      // 验证token是否有效
      api.get('/user/profile')
        .then(() => {
          setIsAuthenticated(true);
        })
        .catch(() => {
          removeToken();
          setIsAuthenticated(false);
          message.error('登录已过期，请重新登录');
        })
        .finally(() => {
          setLoading(false);
        });
    } else {
      setLoading(false);
    }
  }, []);

  const handleLogin = async (username, password) => {
    try {
      const response = await api.post('/auth/login', { username, password });
      const { token } = response.data.data;
      setToken(token);
      setIsAuthenticated(true);
      message.success('登录成功');
      return true;
    } catch (error) {
      message.error(error.response?.data?.error || '登录失败');
      return false;
    }
  };

  const handleLogout = () => {
    removeToken();
    setIsAuthenticated(false);
    message.success('已退出登录');
  };

  if (loading) {
    return <div>加载中...</div>;
  }

  if (!isAuthenticated) {
    return <Login onLogin={handleLogin} />;
  }

  return (
    <MainLayout onLogout={handleLogout}>
                <Routes>
            <Route path="/" element={<Navigate to="/positions" replace />} />
            <Route path="/positions" element={<Positions />} />
            <Route path="/orders" element={<Orders />} />
            <Route path="/trading-pairs" element={<TradingPairs />} />
            <Route path="/balances" element={<Balances />} />
            <Route path="/klines" element={<ChartPage />} />
          </Routes>
    </MainLayout>
  );
}

export default App;
