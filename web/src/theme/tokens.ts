/**
 * 设计令牌：主色走 slate 灰 + teal 青绿，避免"AI 紫蓝"味。
 * tokens 映射到 Naive UI themeOverrides 的结构，供 index.ts 组装 light/dark 两套。
 */
import type { GlobalThemeOverrides } from 'naive-ui'

export const palette = {
  // slate
  slate: {
    50: '#f8fafc',
    100: '#f1f5f9',
    200: '#e2e8f0',
    300: '#cbd5e1',
    400: '#94a3b8',
    500: '#64748b',
    600: '#475569',
    700: '#334155',
    800: '#1e293b',
    900: '#0f172a',
    950: '#020617',
  },
  // teal（青绿主强调色）
  teal: {
    50: '#f0fdfa',
    100: '#ccfbf1',
    200: '#99f6e4',
    300: '#5eead4',
    400: '#2dd4bf',
    500: '#14b8a6',
    600: '#0d9488',
    700: '#0f766e',
    800: '#115e59',
    900: '#134e4a',
  },
} as const

/** 圆角 / 阴影 / 字体层级 token（克制，使用一致刻度） */
export const shape = {
  radius: {
    small: '4px',
    medium: '6px',
    large: '8px',
  },
  shadow: 'none',
  fontFamily:
    "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif",
} as const

/** 页面 body 背景色（浅/深），供全局 JS 同步到 body 元素 */
export function bodyColorFor(theme: 'light' | 'dark'): string {
  return theme === 'dark' ? palette.slate[900] : '#ffffff'
}

/** 卡片背景色（浅/深），与 naive cardColor 对齐 */
export function cardColorFor(theme: 'light' | 'dark'): string {
  return theme === 'dark' ? palette.slate[800] : '#ffffff'
}

/** 一级文本色（浅/深），对应 naive textColor1 */
export function textColor1For(theme: 'light' | 'dark'): string {
  return theme === 'dark' ? palette.slate[100] : palette.slate[900]
}

/** 主强调色（浅/深），对应 naive primaryColor */
export function primaryColorFor(theme: 'light' | 'dark'): string {
  return theme === 'dark' ? palette.teal[400] : palette.teal[600]
}

/** 二级文本色（浅/深），对应 naive textColor2 */
export function textColor2For(theme: 'light' | 'dark'): string {
  return theme === 'dark' ? palette.slate[400] : palette.slate[500]
}

/** 分隔线色（浅/深），对应 naive dividerColor */
export function dividerColorFor(theme: 'light' | 'dark'): string {
  return theme === 'dark' ? palette.slate[700] : palette.slate[200]
}

/** 生成 Naive UI themeOverrides */
export function buildThemeOverrides(theme: 'light' | 'dark'): GlobalThemeOverrides {
  const dark = theme === 'dark'
  const primary = dark ? palette.teal[400] : palette.teal[600]
  const primaryHover = dark ? palette.teal[300] : palette.teal[500]
  const primaryPressed = dark ? palette.teal[500] : palette.teal[700]
  const textColor = dark ? palette.slate[100] : palette.slate[900]
  const textSecondary = dark ? palette.slate[400] : palette.slate[500]

  const base: GlobalThemeOverrides = {
    common: {
      primaryColor: primary,
      primaryColorHover: primaryHover,
      primaryColorPressed: primaryPressed,
      primaryColorSuppl: primaryHover,
      borderRadius: shape.radius.medium,
      textColorBase: textColor,
      textColor1: textColor,
      textColor2: textSecondary,
      fontFamily: shape.fontFamily,
    },
  }

  if (dark) {
    base.common = {
      ...base.common,
      bodyColor: palette.slate[900],
      cardColor: palette.slate[800],
      modalColor: palette.slate[800],
      popoverColor: palette.slate[800],
      inputColor: palette.slate[800],
      hoverColor: palette.slate[700],
      dividerColor: palette.slate[700],
      borderColor: palette.slate[700],
    }
  } else {
    base.common = {
      ...base.common,
      bodyColor: '#ffffff',
      cardColor: '#ffffff',
      modalColor: '#ffffff',
      popoverColor: '#ffffff',
      dividerColor: palette.slate[200],
      borderColor: palette.slate[200],
    }
  }

  return base
}
