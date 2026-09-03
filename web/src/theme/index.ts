/** 主题接入辅助：暴露 Naive UI 所需的 light/dark theme 与 overrides */
import { darkTheme, lightTheme, type GlobalTheme } from 'naive-ui'

import { buildThemeOverrides } from './tokens'

export const themes = {
  light: lightTheme,
  dark: darkTheme,
} as const

export type ThemeName = keyof typeof themes

/** 根据主题名取 Naive UI theme */
export function themeFor(name: ThemeName): GlobalTheme {
  return themes[name]
}

/** 主题 overrides 缓存（构建成本低，直接每次生成即可） */
export function overridesFor(name: ThemeName) {
  return buildThemeOverrides(name)
}

/** localStorage key */
export const THEME_STORAGE_KEY = 'ac_theme'
