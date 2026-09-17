/**
 * demo3d 契约类型（demo_3d 环节）。
 *
 * 原始落库数据走 api/learn.ts 的 Demo3DContent（宽松可选字段），
 * 经 schema.ts 修剪兜底后得到本文件的「确定形态」供渲染器直接消费。
 */

export type Vec3 = [number, number, number]

export interface Demo3DScene {
  background: string
  axesHelper: boolean
}

/** 已修剪的几何体：id/args/变换均为确定值 */
export interface SceneMesh {
  id: string
  type: string
  args: number[]
  position: Vec3
  rotation: Vec3
  scale: Vec3
  /** 材质 id；悬空引用在修剪时已换绑为默认材质 */
  materialId: string
  /**
   * 父几何体 id；null = 挂场景根。
   * 悬空/自指/成环的 parentId 在解析时已剥环挂根；非 null 时 position 为相对父节点的局部坐标。
   */
  parentId: string | null
}

export interface SceneMaterial {
  id: string
  type: string
  color: string
  roughness: number
  metalness: number
  opacity: number
  emissive: string
}

export interface SceneLight {
  type: string
  color: string
  intensity: number
  position: Vec3
  /** 指向型灯光（DirectionalLight/SpotLight）的照射点 */
  target: Vec3
  castShadow: boolean
}

export type SceneCamera = 'PerspectiveCamera' | 'OrthographicCamera'

export type SceneControl =
  | { type: 'orbit'; title: string; autoRotate: boolean }
  | { type: 'rotation' | 'translation' | 'scale'; title: string; targetId: string; axis: 'x' | 'y' | 'z' }

/** 修剪后的完整场景（渲染器输入） */
export interface Demo3DSceneDef {
  scene: Demo3DScene
  meshes: SceneMesh[]
  materials: SceneMaterial[]
  lights: SceneLight[]
  camera: { type: SceneCamera; position: Vec3; fov: number; lookAt: Vec3 }
  controls: SceneControl[]
}

/** 解析整体失败（入参非对象结构） */
export class Demo3DParseError extends Error {}
