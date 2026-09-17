/**
 * demo_3d 场景 JSON 的解析与修剪（纯函数）。
 *
 * 兜底矩阵见 docs/3D演示数据结构.md：未知 geometry/light/材质 type 剔除、
 * 悬空 materialId 换绑默认材质、相机缺省、控制器 targetId 悬空剔除、数值非法取默认。
 * 修剪后的 Demo3DSceneDef 为确定形态，渲染器不再做任何容错判断。
 */

import type { Demo3DContent } from '@/api/learn'
import {
  Demo3DParseError,
  type Demo3DSceneDef,
  type SceneCamera,
  type SceneControl,
  type SceneLight,
  type SceneMaterial,
  type SceneMesh,
  type Vec3,
} from './types'

// ---- 枚举白名单（与《3D演示数据结构.md》一致） ----

const GEOMETRY_TYPES = ['Box', 'Sphere', 'Cylinder', 'Cone', 'Torus', 'TorusKnot', 'Plane', 'Icosahedron']
const MATERIAL_TYPES = [
  'MeshStandardMaterial',
  'MeshBasicMaterial',
  'MeshLambertMaterial',
  'MeshPhongMaterial',
  'MeshPhysicalMaterial',
  'MeshNormalMaterial',
]
const LIGHT_TYPES = ['AmbientLight', 'HemisphereLight', 'DirectionalLight', 'PointLight', 'SpotLight']
const CAMERA_TYPES = ['PerspectiveCamera', 'OrthographicCamera']
const CONTROL_TYPES = ['orbit', 'rotation', 'translation', 'scale']

// ---- 兜底默认值 ----

export const DEFAULT_BACKGROUND = '#0f172a'
export const DEFAULT_CAMERA: Readonly<{ type: SceneCamera; position: Vec3; fov: number; lookAt: Vec3 }> = {
  type: 'PerspectiveCamera',
  position: [5, 5, 5],
  fov: 50,
  lookAt: [0, 0, 0],
}
export const DEFAULT_MATERIAL_COLOR = '#94a3b8'
/** 系统自动补的默认材质 id（悬空引用换绑用） */
export const DEFAULT_MATERIAL_ID = '__default__'
/** 模型变换滑块的系统固定值域 */
export const CONTROL_RANGES = {
  rotation: { min: -180, max: 180, step: 1 },
  translation: { min: -50, max: 50, step: 0.1 },
  scale: { min: 0.01, max: 5, step: 0.01 },
} as const

const clamp = (v: number, min: number, max: number): number => Math.min(max, Math.max(min, v))

function isFinite3(v: unknown, fallback: Vec3): Vec3 {
  return Array.isArray(v) && v.length === 3 && v.every((n) => Number.isFinite(n)) ? ([...v] as Vec3) : fallback
}

function clampVec3(v: Vec3, limit: number): Vec3 {
  return [clamp(v[0], -limit, limit), clamp(v[1], -limit, limit), clamp(v[2], -limit, limit)]
}

/**
 * parentId 拓扑修整：基于原始 parentId 快照逐节点判定。
 * - 悬空 / 空串 / 自指 → 挂根（null）
 * - 沿原始祖先链上溯回到自身（成环）→ 挂根（null）
 * 上溯步数以集合规模为上限（正常链深必小于节点总数）。
 */
function resolveParentId(self: string, rawParents: Map<string, string | null>, ids: Set<string>): string | null {
  const first = rawParents.get(self) ?? null
  if (first === null || first === '' || first === self || !ids.has(first)) return null
  let cur = first
  for (let i = 0; i <= ids.size; i++) {
    if (cur === self) return null
    const next = rawParents.get(cur) ?? null
    if (next === null || next === '' || !ids.has(next)) return first
    cur = next
  }
  return null
}

/** 缩放：数字（整体）或三轴数组，均须 > 0，非法取 1 */
function parseScale(v: unknown): Vec3 {
  const one = (n: unknown): number => (Number.isFinite(n) && (n as number) > 0 ? (n as number) : 1)
  if (typeof v === 'number') {
    const s = one(v)
    return [s, s, s]
  }
  if (Array.isArray(v) && v.length === 3) {
    return [one(v[0]), one(v[1]), one(v[2])]
  }
  return [1, 1, 1]
}

function hexColor(v: unknown, fallback: string): string {
  return typeof v === 'string' && /^#[0-9a-fA-F]{6}$/.test(v) ? v : fallback
}

/** 控制器 title 缺省时按类型 + 目标 id 生成默认文案 */
function controlTitle(type: string, target: string): string {
  switch (type) {
    case 'orbit':
      return '环绕视角'
    case 'rotation':
      return target ? `${target} 自转` : '自转'
    case 'translation':
      return target ? `${target} 平移` : '平移'
    case 'scale':
      return target ? `${target} 缩放` : '缩放'
    default:
      return '控制'
  }
}

/**
 * 解析并修剪演示内容；整体结构性非法（非对象）抛 Demo3DParseError。
 * 除整体失败外任何局部问题都以默认值/剔除处理，不抛错。
 */
export function parseDemo3DContent(raw: unknown): Demo3DSceneDef {
  const content = raw as Demo3DContent | null
  if (!content || typeof content !== 'object') {
    throw new Demo3DParseError('demo_3d content is missing or not an object')
  }

  // ---- 材质：id 缺失/重复剔除；未知 type 兜底 MeshStandardMaterial ----
  const materials: SceneMaterial[] = []
  const materialIdSet = new Set<string>()
  for (const m of content.materials ?? []) {
    if (!m || typeof m !== 'object' || typeof m.id !== 'string' || m.id === '' || materialIdSet.has(m.id)) continue
    materials.push({
      id: m.id,
      type: MATERIAL_TYPES.includes(m.type ?? '') ? (m.type as string) : 'MeshStandardMaterial',
      color: hexColor(m.color, DEFAULT_MATERIAL_COLOR),
      roughness: Number.isFinite(m.roughness) ? clamp(m.roughness as number, 0, 1) : 0.5,
      metalness: Number.isFinite(m.metalness) ? clamp(m.metalness as number, 0, 1) : 0,
      opacity: Number.isFinite(m.opacity) ? clamp(m.opacity as number, 0, 1) : 1,
      emissive: hexColor(m.emissive, '#000000'),
    })
    materialIdSet.add(m.id)
  }

  // ---- 几何体：未知 type / 缺 id / id 重复剔除；materialId 悬空或缺省换绑默认材质 ----
  const meshes: SceneMesh[] = []
  const meshIdSet = new Set<string>()
  let needDefaultMaterial = materials.length === 0
  for (const g of content.geometries ?? []) {
    if (!g || typeof g !== 'object') continue
    if (typeof g.id !== 'string' || g.id === '' || meshIdSet.has(g.id)) continue
    if (typeof g.type !== 'string' || !GEOMETRY_TYPES.includes(g.type)) continue
    const mat = typeof g.materialId === 'string' ? g.materialId : ''
    if (mat !== '' && !materialIdSet.has(mat)) needDefaultMaterial = true
    meshes.push({
      id: g.id,
      type: g.type,
      args: Array.isArray(g.args) ? g.args.filter((n) => Number.isFinite(n)) : [],
      position: clampVec3(isFinite3(g.position, [0, 0, 0]), 10000),
      rotation: isFinite3(g.rotation, [0, 0, 0]),
      scale: parseScale(g.scale),
      materialId: mat !== '' && materialIdSet.has(mat) ? mat : DEFAULT_MATERIAL_ID,
      parentId: typeof g.parentId === 'string' ? g.parentId : null,
    })
    meshIdSet.add(g.id)
  }

  // ---- parentId 拓扑修整：悬空/自指/成环一律挂根（null），不触发后端重试 ----
  const rawParents = new Map(meshes.map((m) => [m.id, m.parentId]))
  for (const mesh of meshes) {
    mesh.parentId = resolveParentId(mesh.id, rawParents, meshIdSet)
  }

  // ---- 默认材质：存在悬空引用或缺材质集合时补入 ----
  if (needDefaultMaterial) {
    materials.push({
      id: DEFAULT_MATERIAL_ID,
      type: 'MeshStandardMaterial',
      color: DEFAULT_MATERIAL_COLOR,
      roughness: 0.5,
      metalness: 0,
      opacity: 1,
      emissive: '#000000',
    })
    materialIdSet.add(DEFAULT_MATERIAL_ID)
  }

  // ---- 灯光：未知 type 剔除 ----
  const lights: SceneLight[] = (content.lights ?? [])
    .filter((l): l is NonNullable<typeof l> => !!l && typeof l === 'object' && typeof l.type === 'string' && LIGHT_TYPES.includes(l.type))
    .map((l) => ({
      type: l.type as string,
      color: hexColor(l.color, '#ffffff'),
      intensity: Number.isFinite(l.intensity) ? Math.max(0, l.intensity as number) : 1,
      position: isFinite3(l.position, [0, 0, 0]),
      target: isFinite3(l.target, [0, 0, 0]),
      castShadow: l.castShadow === true,
    }))

  // ---- 相机：有且取第一个；缺失/类型未知用默认 ----
  const first = content.cameras?.[0]
  const camera = first && first.type && CAMERA_TYPES.includes(first.type)
    ? {
        type: first.type as SceneCamera,
        position: clampVec3(isFinite3(first.position, DEFAULT_CAMERA.position), 10000),
        fov: Number.isFinite(first.fov) ? clamp(first.fov as number, 1, 180) : DEFAULT_CAMERA.fov,
        lookAt: isFinite3(first.lookAt, [0, 0, 0]),
      }
    : {
        type: DEFAULT_CAMERA.type,
        position: [...DEFAULT_CAMERA.position] as Vec3,
        fov: DEFAULT_CAMERA.fov,
        lookAt: [...DEFAULT_CAMERA.lookAt] as Vec3,
      }

  // ---- 控制器：type/targetId 非法剔除；orbit 可省略 targetId ----
  const controls: SceneControl[] = []
  for (const c of content.controls ?? []) {
    if (!c || typeof c !== 'object') continue
    const type = c.type
    if (typeof type !== 'string' || !CONTROL_TYPES.includes(type)) continue
    const title =
      typeof c.title === 'string' && c.title.trim() !== ''
        ? c.title
        : controlTitle(type, typeof c.targetId === 'string' ? c.targetId : '')
    if (type === 'orbit') {
      controls.push({ type: 'orbit', title, autoRotate: c.autoRotate === true })
      continue
    }
    const targetId = typeof c.targetId === 'string' ? c.targetId : ''
    if (!meshIdSet.has(targetId)) continue
    const axis = c.axis === 'x' || c.axis === 'z' ? c.axis : 'y'
    controls.push({ type, title, targetId, axis } as SceneControl)
  }

  return {
    scene: {
      background: hexColor(content.scene?.background, DEFAULT_BACKGROUND),
      axesHelper: content.scene?.axesHelper === true,
    },
    meshes,
    materials,
    lights,
    camera,
    controls,
  }
}
