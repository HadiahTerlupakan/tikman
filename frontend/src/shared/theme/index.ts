import type { ThemeConfig } from 'antd';

export const theme: ThemeConfig = {
  token: {
    colorPrimary: '#10b981',
    colorSuccess: '#10b981',
    colorWarning: '#f59e0b',
    colorError: '#ef4444',
    colorInfo: '#3b82f6',
    colorTextBase: '#0f172a',
    colorBgBase: '#ffffff',
    borderRadius: 8,
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
    fontSize: 14,
    controlHeight: 40,
  },
  components: {
    Layout: {
      headerBg: '#ffffff',
      headerHeight: 64,
      headerPadding: '0 24px',
      siderBg: '#0f172a',
      bodyBg: '#f8fafc',
    },
    Menu: {
      itemBg: 'transparent',
      itemSelectedBg: 'rgba(16, 185, 129, 0.1)',
      itemSelectedColor: '#10b981',
      itemHoverBg: 'rgba(255, 255, 255, 0.08)',
      itemHoverColor: '#ffffff',
      itemColor: '#94a3b8',
      itemActiveBg: 'rgba(16, 185, 129, 0.15)',
      iconSize: 18,
      itemHeight: 44,
      itemMarginBlock: 4,
      itemMarginInline: 8,
      itemBorderRadius: 8,
    },
    Card: {
      borderRadiusLG: 12,
      paddingLG: 20,
      boxShadowTertiary: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
    },
    Table: {
      headerBg: '#f8fafc',
      headerColor: '#475569',
      rowHoverBg: '#f1f5f9',
      borderColor: '#e2e8f0',
      cellPaddingBlock: 16,
      cellPaddingInline: 16,
    },
    Button: {
      borderRadius: 8,
      controlHeight: 40,
      fontWeight: 500,
      primaryShadow: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
    },
    Input: {
      borderRadius: 8,
      controlHeight: 40,
    },
    Select: {
      borderRadius: 8,
      controlHeight: 40,
    },
  },
};
