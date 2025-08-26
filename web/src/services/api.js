import axios from 'axios';
import { getToken, removeToken } from '../utils/auth';

// 创建axios实例
const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
});

// 请求拦截器 - 添加token
api.interceptors.request.use(
  (config) => {
    const token = getToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器 - 处理token过期
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      removeToken();
      window.location.href = '/';
    }
    return Promise.reject(error);
  }
);

// 获取价格预估列表（仅监听状态）
export const getEstimates = async (symbol = null, status = null) => {
  try {
    let url = '/estimates';
    const params = new URLSearchParams();
    
    if (symbol) params.append('symbol', symbol);
    if (status) params.append('status', status);
    
    if (params.toString()) {
      url += `?${params.toString()}`;
    }
    
    const response = await api.get(url);
    return response.data.data || [];
  } catch (error) {
    console.error('获取价格预估列表失败:', error);
    return [];
  }
};

// 获取所有状态的价格预估列表（包括triggered, failed）
export const getAllEstimates = async (symbol = null) => {
  try {
    let url = '/estimates/all';
    const params = new URLSearchParams();
    
    if (symbol) params.append('symbol', symbol);
    
    if (params.toString()) {
      url += `?${params.toString()}`;
    }
    
    const response = await api.get(url);
    return response.data.data || [];
  } catch (error) {
    console.error('获取所有价格预估列表失败:', error);
    return [];
  }
};

// 删除价格预估
export const deleteEstimate = async (id) => {
  try {
    const response = await api.delete(`/estimates/${id}`);
    return response.data;
  } catch (error) {
    console.error('删除价格预估失败:', error);
    throw error;
  }
};



// 切换价格监听状态
export const toggleEstimateEnabled = async (id, enabled) => {
  try {
    const payload = { enabled };
    const response = await api.put(`/estimates/${id}/toggle`, payload);
    return response.data;
  } catch (error) {
    console.error('切换监听状态失败:', error.response?.data?.error || error.message);
    throw error;
  }
};

// 批量获取所有选中币种的价格数据
export const getAllSelectedCoinsPrices = async () => {
  try {
    const response = await api.get('/prices/selected');
    return response.data.data || [];
  } catch (error) {
    console.error('批量获取价格数据失败:', error);
    return [];
  }
};

export default api;
