import React from 'react';
import { Drawer, Form, Typography, Tag, Select, InputNumber, Slider } from 'antd';
import { 
  getInputStep,
  getInputPrecision
} from '../../utils/precision';

const { Text } = Typography;

/**
 * 交易抽屉组件
 * @param {boolean} visible - 是否显示
 * @param {Function} onClose - 关闭回调
 * @param {string} symbol - 交易对
 * @param {string} side - 交易方向
 * @param {Object} form - 表单实例
 * @param {Function} onSubmit - 提交回调
 * @param {Object} priceData - 价格数据
 * @param {Object} precisionInfo - 精度信息
 * @param {Object} accountValue - 账户信息
 * @param {number} targetPrice - 目标价格
 * @param {Function} onPriceChange - 价格变化回调
 * @param {number} coinQuantity - 币数量
 * @param {Function} onQuantityChange - 数量变化回调
 * @param {number} usdtAmount - USDT金额
 * @param {string} orderType - 订单类型
 * @param {Function} onOrderTypeChange - 订单类型变化回调
 * @param {number} selectedLeverage - 杠杆
 * @param {Function} onLeverageChange - 杠杆变化回调
 * @param {Function} getMaxCoinQuantity - 获取最大币数量
 * @param {Function} getPriceRange - 获取价格范围
 * @param {Function} handleSliderChange - 滑块变化处理
 * @param {Function} calculateUsdtRatio - 计算USDT比例
 */
const TradeDrawer = ({
  visible,
  onClose,
  symbol,
  side,
  form,
  onSubmit,
  priceData,
  precisionInfo,
  accountValue,
  targetPrice,
  onPriceChange,
  coinQuantity,
  onQuantityChange,
  usdtAmount,
  orderType,
  onOrderTypeChange,
  selectedLeverage,
  onLeverageChange,
  getMaxCoinQuantity,
  getPriceRange,
  handleSliderChange,
  calculateUsdtRatio
}) => {
  const calculatePriceDifference = () => {
    const currentPrice = priceData[symbol];
    if (!currentPrice?.hasValidData || !targetPrice) return 0;
    
    const referencePrice = currentPrice.currentPrice;
    return ((targetPrice - referencePrice) / referencePrice * 100);
  };

  return (
    <Drawer
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Tag color={side === 'long' ? 'green' : 'red'}>
            {side === 'long' ? '开多' : '开空'}
          </Tag>
          <span>{symbol}</span>
        </div>
      }
      open={visible}
      onClose={onClose}
      width={window.innerWidth < 768 ? '100%' : 480}
      placement="right"
      destroyOnClose={true}
      extra={
        <button 
          type="submit"
          form="trade-form"
          className={`drawer-confirm-btn ${side === 'long' ? 'success' : 'danger'}`}
        >
          确认{side === 'long' ? '开多' : '开空'}
        </button>
      }
      styles={{
        body: {
          paddingTop: 0
        }
      }}
    >
      <Form
        id="trade-form"
        form={form}
        layout="vertical"
        onFinish={onSubmit}
        initialValues={{
          marginMode: 'isolated',
          leverage: 3,
          orderType: 'limit',
          targetPrice: 0
        }}
      >
        {/* 交易信息摘要 */}
        <div style={{ marginBottom: 16, padding: 12, backgroundColor: '#fafafa', borderRadius: 6 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">交易对:</Text>
            <Text strong>{symbol}</Text>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">操作:</Text>
            <Tag color={side === 'long' ? 'green' : 'red'}>
              {side === 'long' ? '开多 ↗' : '开空 ↘'}
            </Tag>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">杠杆:</Text>
            <Tag color="blue">{selectedLeverage}x</Tag>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">订单类型:</Text>
            <Tag color={orderType === 'market' ? 'green' : 'blue'}>
              {orderType === 'market' ? '市价单' : '限价单'}
            </Tag>
          </div>
          {priceData[symbol]?.hasValidData && (
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
              <Text type="secondary">
                {orderType === 'market' ? '当前市价:' : '限价价格:'}
              </Text>
              <Text strong>
                ${orderType === 'market'
                  ? (side === 'long' 
                    ? priceData[symbol].bestAsk?.toFixed(4) 
                    : priceData[symbol].bestBid?.toFixed(4))
                  : targetPrice?.toFixed(4) || '未设置'
                }
              </Text>
            </div>
          )}
          {precisionInfo[symbol] && (
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <Text type="secondary">精度要求:</Text>
              <Text type="secondary" style={{ fontSize: '12px' }}>
                价格步长 {precisionInfo[symbol].tick_size} | 数量步长 {precisionInfo[symbol].step_size}
              </Text>
            </div>
          )}
        </div>

        {/* 杠杆选择 */}
        <Form.Item
          label="杠杆倍数"
          name="leverage"
          rules={[{ required: true, message: '请选择杠杆倍数' }]}
        >
          <Select 
            placeholder="选择杠杆倍数"
            onChange={onLeverageChange}
          >
            <Select.Option value={1}>1x (无杠杆)</Select.Option>
            <Select.Option value={2}>2x</Select.Option>
            <Select.Option value={3}>3x</Select.Option>
            <Select.Option value={4}>4x</Select.Option>
            <Select.Option value={5}>5x</Select.Option>
          </Select>
        </Form.Item>

        {/* 订单类型选择 */}
        <Form.Item
          label="订单类型"
          name="orderType"
          rules={[{ required: true, message: '请选择订单类型' }]}
        >
          <Select 
            placeholder="选择订单类型"
            onChange={onOrderTypeChange}
          >
            <Select.Option value="market">
              <strong>市价单</strong>
            </Select.Option>
            <Select.Option value="limit">
              <strong>限价单</strong>
            </Select.Option>
          </Select>
        </Form.Item>

        {/* 限价单才显示价格设置 */}
        {orderType === 'limit' && (
          <>
            <Form.Item
              label={
                <div>
                  限价价格 {targetPrice > 0 && (
                    <Tag color={calculatePriceDifference() > 0 ? 'red' : 'green'} size="small">
                      {calculatePriceDifference() > 0 ? '+' : ''}{calculatePriceDifference().toFixed(2)}%
                    </Tag>
                  )}
                </div>
              }
              name="targetPrice"
              rules={[
                { required: orderType === 'limit', message: '请输入限价价格' },
                { type: 'number', min: 0.0001, message: '价格必须大于0' }
              ]}
            >
              <InputNumber
                style={{ width: '100%' }}
                placeholder="请输入限价价格"
                step={precisionInfo[symbol] ? getInputStep(precisionInfo[symbol].tick_size) : 0.01}
                precision={precisionInfo[symbol] ? getInputPrecision(precisionInfo[symbol].price_precision) : 4}
                min={precisionInfo[symbol] ? parseFloat(precisionInfo[symbol].min_price) : 0.0001}
                max={precisionInfo[symbol] ? parseFloat(precisionInfo[symbol].max_price) : undefined}
                value={targetPrice}
                onChange={onPriceChange}
                addonBefore="$"
              />
            </Form.Item>

            {/* 价格滑动条 - 仅限价单显示 */}
            {priceData[symbol]?.hasValidData && (
              <Form.Item label="价格调整">
                <Slider
                  range={false}
                  min={getPriceRange()[0]}
                  max={getPriceRange()[1]}
                  step={precisionInfo[symbol] ? 
                    parseFloat(precisionInfo[symbol].tick_size) : 
                    getPriceRange()[0] * 0.0001
                  }
                  value={targetPrice}
                  onChange={handleSliderChange}
                  tooltip={{
                    formatter: (value) => `$${value?.toFixed(precisionInfo[symbol] ? precisionInfo[symbol].price_precision : 4)}`
                  }}
                  marks={{
                    [priceData[symbol].currentPrice]: {
                      style: { color: '#1890ff' },
                      label: '当前价格'
                    }
                  }}
                />
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: '#999', marginTop: 4 }}>
                  <span>-20%</span>
                  <span>当前价格: ${priceData[symbol].currentPrice?.toFixed(4)}</span>
                  <span>+20%</span>
                  {precisionInfo[symbol] && (
                    <span>步长: {precisionInfo[symbol].tick_size}</span>
                  )}
                </div>
                {precisionInfo[symbol] && (
                  <div style={{ fontSize: '12px', color: '#666', textAlign: 'center', marginTop: 4 }}>
                    💡 滑块步长已按 {precisionInfo[symbol].tick_size} 设置
                  </div>
                )}
              </Form.Item>
            )}
          </>
        )}

        {/* 币数量滑块 */}
        <Form.Item
          label={
            <div>
              {symbol.replace('USDT', '')} 数量
              <Text type="secondary" style={{ fontSize: '12px', marginLeft: 8 }}>
                最大: {getMaxCoinQuantity().toFixed(6)}
              </Text>
            </div>
          }
        >
          <Slider
            min={precisionInfo[symbol] ? parseFloat(precisionInfo[symbol].min_qty) : 0.001}
            max={getMaxCoinQuantity()}
            step={precisionInfo[symbol] ? parseFloat(precisionInfo[symbol].step_size) : 0.001}
            value={coinQuantity}
            onChange={onQuantityChange}
            tooltip={{
              formatter: (value) => `${value?.toFixed(precisionInfo[symbol] ? precisionInfo[symbol].quantity_precision : 6)} ${symbol.replace('USDT', '')}`
            }}
            marks={{
              [getMaxCoinQuantity()]: {
                style: { color: '#1890ff' },
                label: '最大'
              }
            }}
          />
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: '#999', marginTop: 4 }}>
            <span>最小: {precisionInfo[symbol] ? precisionInfo[symbol].min_qty : '0.001'}</span>
            <span>当前: {coinQuantity.toFixed(6)}</span>
            <span>最大: {getMaxCoinQuantity().toFixed(6)}</span>
          </div>
          {precisionInfo[symbol] && (
            <div style={{ fontSize: '12px', color: '#666', textAlign: 'center', marginTop: 4 }}>
              💡 滑块步长已按 {precisionInfo[symbol].step_size} 设置
            </div>
          )}
        </Form.Item>

        {/* USDT金额显示（只读） */}
        <Form.Item
          label={
            <div>
              USDT金额
              <Tag color="blue" size="small" style={{ marginLeft: 8 }}>
                {calculateUsdtRatio().toFixed(1)}%
              </Tag>
              <Text type="secondary" style={{ fontSize: '12px', marginLeft: 8 }}>
                可用: {accountValue.usdt_free.toLocaleString()} USDT
              </Text>
            </div>
          }
        >
          <div style={{ 
            padding: '8px 12px', 
            backgroundColor: '#f5f5f5', 
            border: '1px solid #d9d9d9',
            borderRadius: '6px',
            fontSize: '16px',
            fontWeight: 'bold',
            color: '#262626',
            textAlign: 'center'
          }}>
            {usdtAmount.toFixed(2)} USDT
          </div>
          <Text type="secondary" style={{ fontSize: '12px', display: 'block', marginTop: 4 }}>
            💡 根据币数量、价格和杠杆自动计算
          </Text>
        </Form.Item>

        {/* 保证金模式 */}
        <Form.Item
          label="保证金模式"
          name="marginMode"
        >
          <Select>
            <Select.Option value="cross">全仓模式</Select.Option>
            <Select.Option value="isolated">逐仓模式</Select.Option>
          </Select>
        </Form.Item>


      </Form>
    </Drawer>
  );
};

export default TradeDrawer;
