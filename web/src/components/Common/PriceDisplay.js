import React from 'react';
import { Typography, Spin } from 'antd';

const { Text } = Typography;

/**
 * 通用价格显示组件
 * @param {Object} priceData - 价格数据
 * @param {boolean} loading - 加载状态
 * @param {string} symbol - 交易对符号
 */
const PriceDisplay = ({ priceData, loading, symbol }) => {
  if (loading) {
    return (
      <div style={{ height: '140px', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="small" />
        <Text type="secondary" style={{ fontSize: '12px', marginTop: 8 }}>
          获取价格中...
        </Text>
      </div>
    );
  }

  if (!priceData?.hasValidData) {
    return (
      <div style={{ height: '140px', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
        <Text type="secondary" style={{ fontSize: '12px', color: '#ff4d4f' }}>
          ⚠️ 订单薄数据为空
        </Text>
        <Text type="secondary" style={{ fontSize: '10px' }}>
          WebSocket可能未连接
        </Text>
      </div>
    );
  }

  const { currentPrice, bestBid, bestAsk } = priceData;
  const spread = ((bestAsk - bestBid) / bestBid * 100).toFixed(3);

  return (
    <div>
      {/* 主要价格 */}
      <div style={{ marginBottom: 12, textAlign: 'center' }}>
        <Text strong style={{ fontSize: '20px', color: '#262626', fontWeight: 700 }}>
          ${currentPrice.toFixed(4)}
        </Text>
      </div>
      
      {/* 买卖价格 */}
      <div className="price-section">
        <div style={{ 
          display: 'flex', 
          justifyContent: 'space-between', 
          alignItems: 'flex-start',
          gap: '6px',
          marginBottom: 8
        }}>
          <div style={{ flex: 1, textAlign: 'left', minWidth: '45%', maxWidth: '48%' }}>
            <Text type="secondary" style={{ fontSize: '10px', display: 'block', marginBottom: '3px' }}>买一:</Text>
            <div className="bid-price" style={{ fontSize: '11px' }}>
              {bestBid.toFixed(4)}
            </div>
          </div>
          <div style={{ flex: 1, textAlign: 'right', minWidth: '45%', maxWidth: '48%' }}>
            <Text type="secondary" style={{ fontSize: '10px', display: 'block', marginBottom: '3px' }}>卖一:</Text>
            <div className="ask-price" style={{ fontSize: '11px' }}>
              {bestAsk.toFixed(4)}
            </div>
          </div>
        </div>
        
        <div style={{ fontSize: '10px', textAlign: 'center', marginTop: 6, paddingTop: 6, borderTop: '1px solid #f0f0f0' }}>
          <Text type="secondary">
            价差: {spread}%
          </Text>
        </div>
      </div>
    </div>
  );
};

export default PriceDisplay;
