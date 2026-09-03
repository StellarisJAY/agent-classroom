import { beforeEach, describe, expect, it } from 'vitest'

import { clearToken, getToken, setToken } from '@/api/token'

describe('token storage', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns null when no token stored', () => {
    expect(getToken()).toBeNull()
  })

  it('persists and clears token', () => {
    setToken('abc.def.ghi')
    expect(getToken()).toBe('abc.def.ghi')

    clearToken()
    expect(getToken()).toBeNull()
  })

  it('overwrites an existing token', () => {
    setToken('first')
    setToken('second')
    expect(getToken()).toBe('second')
  })
})
