import React, { useEffect } from 'react';
import { notification, Tag } from 'antd';
import { 
  CheckCircleOutlined, 
  DollarOutlined,
  TrophyOutlined,
  ShoppingCartOutlined
} from '@ant-design/icons';
import WebSocketManager from '../../utils/websocket';

// 事件类型配置
const EVENT_CONFIG = {
  balance_update: {
    title: '余额更新',
    icon: <DollarOutlined style={{ color: '#52c41a' }} />,
    color: '#52c41a',
    tag: '余额',
  },
  position_update: {
    title: '持仓更新',
    icon: <TrophyOutlined style={{ color: '#1890ff' }} />,
    color: '#1890ff',
    tag: '持仓',
  },
  order_update: {
    title: '订单更新',
    icon: <ShoppingCartOutlined style={{ color: '#722ed1' }} />,
    color: '#722ed1',
    tag: '订单',
  },
  estimate_trigger: {
    title: '预估触发',
    icon: <CheckCircleOutlined style={{ color: '#f5222d' }} />,
    color: '#f5222d',
    tag: '触发',
  },
};

const EventNotification = () => {
  // 处理WebSocket事件消息
  const handleEventMessage = React.useCallback((eventData) => {
    try {
      const { event_type, data } = eventData;
      const config = EVENT_CONFIG[event_type];
      
      if (!config) {
        console.warn('[事件通知] 未知事件类型:', event_type);
        return;
      }

      // 显示通知
      showNotification(config, data);

    } catch (error) {
      console.error('[事件通知] 处理事件失败:', error);
    }
  }, []);

  // 显示通知
  const showNotification = (config, eventData) => {
    const { title, icon, color, tag } = config;
    const message = eventData.message || title;

    notification.open({
      message: (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {icon}
          <span>{title}</span>
          <Tag color={color} size="small">{tag}</Tag>
        </div>
      ),
      description: message,
      placement: 'topRight',
      duration: 4, // 4秒后自动关闭
      style: {
        minWidth: 300,
      },
    });
  };

  // WebSocket连接管理
  useEffect(() => {
    // 连接状态处理函数
    const handleConnect = () => {
      // 连接成功，无需特殊处理
    };

    const handleDisconnect = () => {
      // 连接断开，无需特殊处理
    };

    const initializeEventListener = () => {
      // 设置连接状态监听
      WebSocketManager.addConnectionListener(handleConnect);
      WebSocketManager.addDisconnectionListener(handleDisconnect);

      // 订阅事件消息
      WebSocketManager.subscribe('events', handleEventMessage);

      // 检查当前连接状态
      WebSocketManager.getStatus();
    };

    initializeEventListener();

    // 清理函数
    return () => {
      WebSocketManager.unsubscribe('events', handleEventMessage);
      WebSocketManager.removeConnectionListener(handleConnect);
      WebSocketManager.removeDisconnectionListener(handleDisconnect);
    };
  }, [handleEventMessage]);



  // 这个组件不渲染UI，只处理事件监听
  return null;
};

export default EventNotification;
