import { describe, expect, it } from 'vitest'

import {
  DEFAULT_CAMERA,
  DEFAULT_MATERIAL_ID,
  parseDemo3DContent,
} from '@/components/learn/demo3d/schema'
import { Demo3DParseError } from '@/components/learn/demo3d/types'

import { DEMO3D_CONTENT } from '@/api/learn.mock'

const validContent = structuredClone(DEMO3D_CONTENT)

describe('parseDemo3DContent', () => {
  it('合法内容完整解析：mesh/材质/灯光/相机/控制器数量保持', () => {
    const def = parseDemo3DContent(structuredClone(validContent))
    expect(def.meshes).toHaveLength(5)
    expect(def.materials.map((m) => m.id)).toEqual(['mat-o', 'mat-h', 'mat-bond'])
    expect(def.lights).toHaveLength(2)
    expect(def.camera.type).toBe('PerspectiveCamera')
    expect(def.controls).toHaveLength(2)
    // rotation 滑块初值取 mesh 当前旋转角换算角度
    const rot = def.controls.find((c) => c.type === 'rotation')
    expect(rot).toEqual({ type: 'rotation', title: '氧原子自转', targetId: 'atom-o', axis: 'y' })
  })

  it('整体非对象抛 Demo3DParseError', () => {
    expect(() => parseDemo3DContent(null)).toThrow(Demo3DParseError)
    expect(() => parseDemo3DContent('str')).toThrow(Demo3DParseError)
  })

  it('空对象也给出可用默认场景', () => {
    const def = parseDemo3DContent({})
    expect(def.meshes).toHaveLength(0)
    expect(def.camera).toEqual({ ...DEFAULT_CAMERA })
    expect(def.lights).toHaveLength(0)
    expect(def.controls).toHaveLength(0)
  })

  it('未知几何体类型剔除，已知类型保留', () => {
    const def = parseDemo3DContent({
      geometries: [
        { id: 'a', type: 'Box' },
        { id: 'b', type: 'SomeForeignMesh' },
      ],
    })
    expect(def.meshes.map((m) => m.id)).toEqual(['a'])
  })

  it('悬空 materialId 换绑默认材质，并可正常归位合法引用', () => {
    const def = parseDemo3DContent({
      materials: [{ id: 'm1', type: 'MeshStandardMaterial', color: '#ff0000' }],
      geometries: [
        { id: 'a', type: 'Box', materialId: 'm1' },
        { id: 'b', type: 'Box', materialId: 'missing' },
        { id: 'c', type: 'Box' },
      ],
    })
    expect(def.meshes[0]!.materialId).toBe('m1')
    expect(def.meshes[1]!.materialId).toBe(DEFAULT_MATERIAL_ID)
    expect(def.meshes[2]!.materialId).toBe(DEFAULT_MATERIAL_ID)
    expect(def.materials.map((m) => m.id)).toEqual(['m1', DEFAULT_MATERIAL_ID])
  })

  it('未知材质类型兜底 MeshStandardMaterial', () => {
    const def = parseDemo3DContent({ materials: [{ id: 'm', type: 'StrangeMaterial' }], geometries: [] })
    expect(def.materials[0]!.type).toBe('MeshStandardMaterial')
  })

  it('相机缺失 / 类型未知 → 默认透视相机', () => {
    expect(parseDemo3DContent({ cameras: [] }).camera).toEqual({ ...DEFAULT_CAMERA })
    expect(parseDemo3DContent({ cameras: [{ type: 'ForeignCamera' }] }).camera).toEqual({ ...DEFAULT_CAMERA })
  })

  it('相机列表取第一个，fov 越界 clamp', () => {
    const def = parseDemo3DContent({
      cameras: [
        { type: 'PerspectiveCamera', fov: 800 },
        { type: 'OrthographicCamera' },
      ],
    })
    expect(def.camera.type).toBe('PerspectiveCamera')
    expect(def.camera.fov).toBe(180)
  })

  it('未知灯光类型剔除，position/target 非法取默认值', () => {
    const def = parseDemo3DContent({
      lights: [
        { type: 'AmbientLight', intensity: -2 },
        { type: 'CandleLight', intensity: 1 },
        { type: 'DirectionalLight', position: [1, 'x', 3] },
      ],
    })
    expect(def.lights).toHaveLength(2)
    expect(def.lights[0]!.intensity).toBe(0)
    expect(def.lights[1]!.position).toEqual([0, 0, 0])
  })

  it('scale 兼容数字（整体）与三轴（各轴）两种形态，零值回退 1', () => {
    const def = parseDemo3DContent({
      geometries: [
        { id: 'a', type: 'Box', scale: 2 },
        { id: 'b', type: 'Box', scale: [1, 0, 3] },
        { id: 'c', type: 'Box' },
      ],
    })
    expect(def.meshes[0]!.scale).toEqual([2, 2, 2])
    expect(def.meshes[1]!.scale).toEqual([1, 1, 3])
    expect(def.meshes[2]!.scale).toEqual([1, 1, 1])
  })

  it('控制器：未知 type / 悬空 targetId 剔除；axis 非法回退 y；title 缺省自动生成', () => {
    const def = parseDemo3DContent({
      geometries: [{ id: 'a', type: 'Box' }],
      controls: [
        { type: 'orbit', title: '环绕' },
        { type: 'rotation', targetId: 'missing', axis: 'x' },
        { type: 'scale', targetId: 'a', axis: 'q' },
        { type: 'translation', targetId: 'a', axis: 'z' },
        { type: 'squeeze', targetId: 'a' },
      ],
    })
    expect(def.controls).toHaveLength(3)
    expect(def.controls[0]).toEqual({ type: 'orbit', title: '环绕', autoRotate: false })
    expect(def.controls[1]).toEqual({ type: 'scale', title: 'a 缩放', targetId: 'a', axis: 'y' })
    expect(def.controls[2]).toEqual({ type: 'translation', title: 'a 平移', targetId: 'a', axis: 'z' })
  })

  it('scene 背景色非法回退默认深色', () => {
    expect(parseDemo3DContent({ scene: { background: 'red' } }).scene.background).toBe('#0f172a')
    expect(parseDemo3DContent({}).scene.background).toBe('#0f172a')
  })

  it('parentId 合法链保留，局部坐标语义透传由渲染器组装', () => {
    const def = parseDemo3DContent({
      geometries: [
        { id: 'root', type: 'Box', position: [100, 0, 0] },
        { id: 'child', type: 'Box', position: [1, 2, 3], parentId: 'root' },
        { id: 'grand', type: 'Box', parentId: 'child' },
      ],
    })
    expect(def.meshes[0]!.parentId).toBeNull()
    expect(def.meshes[1]!.parentId).toBe('root')
    expect(def.meshes[2]!.parentId).toBe('child')
    expect(def.meshes[1]!.position).toEqual([1, 2, 3])
  })

  it('parentId 悬空 → 挂根；空串 → 挂根', () => {
    const def = parseDemo3DContent({
      geometries: [
        { id: 'a', type: 'Box', parentId: 'missing' },
        { id: 'b', type: 'Box', parentId: '' },
      ],
    })
    expect(def.meshes[0]!.parentId).toBeNull()
    expect(def.meshes[1]!.parentId).toBeNull()
  })

  it('parentId 自指 → 挂根；两节点成环均剥环挂根', () => {
    const def = parseDemo3DContent({
      geometries: [
        { id: 'a', type: 'Box', parentId: 'a' },
        { id: 'b', type: 'Box', parentId: 'c' },
        { id: 'c', type: 'Box', parentId: 'b' },
      ],
    })
    expect(def.meshes.map((m) => m.parentId)).toEqual([null, null, null])
  })

  it('祖先链尾悬空仅剥离该链上溯起点之上部分：直接父合法则保留', () => {
    const def = parseDemo3DContent({
      geometries: [
        { id: 'root', type: 'Box' },
        { id: 'mid', type: 'Box', parentId: 'ghost' },
        { id: 'leaf', type: 'Box', parentId: 'mid' },
      ],
    })
    // leaf → mid 合法保留；mid 的悬空父挂根（leaf 挂 mid 不受影响）
    expect(def.meshes[1]!.parentId).toBeNull()
    expect(def.meshes[2]!.parentId).toBe('mid')
  })

  it('合法示例（mock 水分子）原子/键均挂氧原子', () => {
    const def = parseDemo3DContent(structuredClone(DEMO3D_CONTENT))
    for (const mesh of def.meshes) {
      expect(mesh.parentId === null || mesh.parentId === 'atom-o').toBe(true)
    }
  })
})
