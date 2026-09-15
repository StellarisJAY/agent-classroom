import { onBeforeUnmount, readonly, ref } from 'vue'

// 移动端断点：与 CSS 中的 @media (max-width: 768px) 保持一致
export const MOBILE_BREAKPOINT = 768

function useMediaQuery(query: string, initial: boolean) {
  const matches = ref(initial)
  let mql: MediaQueryList | null = null
  let handler: ((e: MediaQueryListEvent) => void) | null = null

  if (typeof window !== 'undefined' && 'matchMedia' in window) {
    mql = window.matchMedia(query)
    matches.value = mql.matches
    handler = (e) => {
      matches.value = e.matches
    }
    mql.addEventListener('change', handler)
  }

  onBeforeUnmount(() => {
    if (mql && handler) mql.removeEventListener('change', handler)
  })

  return readonly(matches)
}

/** 是否处于移动断点（宽度 ≤ 768px，含小屏横屏） */
export function useIsMobile() {
  return useMediaQuery(`(max-width: ${MOBILE_BREAKPOINT}px)`, false)
}
