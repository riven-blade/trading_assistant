import React from 'react';
import { Space, Button } from 'antd';

/**
 * 通用页面头部组件
 * @param {string} title - 页面标题
 * @param {Array} actions - 操作按钮数组
 * @param {React.Node} extra - 额外内容
 */
const PageHeader = ({ title, actions = [], extra }) => {
  return (
    <div style={{ 
      display: 'flex', 
      justifyContent: 'space-between', 
      alignItems: 'center', 
      marginBottom: 24,
      flexWrap: 'wrap',
      gap: '16px'
    }}>
      <div className="page-title-clean">{title}</div>
      <Space size="middle" wrap>
        {actions.map((action, index) => (
          React.isValidElement(action) ? (
            action
          ) : (
            <Button 
              key={index}
              {...action}
            />
          )
        ))}
        {extra}
      </Space>
    </div>
  );
};

export default PageHeader;
