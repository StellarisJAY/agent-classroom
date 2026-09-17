/**
 * Three.js 场景构建器：输入修剪后的 Demo3DSceneDef，内建 renderer / scene /
 * 相机 / 灯光 / 几何体与控制器，输出统一实例（dispose 自释放）。
 *
 * 与《3D演示数据结构.md》渲染器行为约定的对应：
 * - 场景初始化（renderer/scene/相机）由本模块完成；
 * - orbit → OrbitControls；rotation/translation/scale → 绝对值赋值（零状态）；
 * - 多个控制器并存，各控制一个 mesh 的一根轴；
 * - autoRotate / castShadow 仅存储声明，当前版本不实现自转与阴影；
 * - ResizeObserver 自适应容器，dispose 全量释放。
 */

import {
  AmbientLight,
  AxesHelper,
  BoxGeometry,
  Color,
  ConeGeometry,
  CylinderGeometry,
  DirectionalLight,
  GridHelper,
  HemisphereLight,
  IcosahedronGeometry,
  MathUtils,
  Mesh,
  MeshBasicMaterial,
  MeshLambertMaterial,
  MeshNormalMaterial,
  MeshPhongMaterial,
  MeshPhysicalMaterial,
  MeshStandardMaterial,
  Object3D,
  OrthographicCamera,
  PerspectiveCamera,
  PlaneGeometry,
  PointLight,
  Scene,
  SphereGeometry,
  SpotLight,
  TorusGeometry,
  TorusKnotGeometry,
  Vector3,
  WebGLRenderer,
  type BufferGeometry,
  type Material,
} from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { CONTROL_RANGES, DEFAULT_MATERIAL_ID } from './schema'
import type { Demo3DSceneDef, SceneControl, SceneLight, SceneMaterial, SceneMesh, Vec3 } from './types'

/** 供 UI 面板读写的滑块定义 */
export interface SliderDef {
  control: SceneControl
  min: number
  max: number
  step: number
  /** 初始绝对值（rotation 为角度，translation 为世界坐标，scale 恒为 1） */
  initial: number
}

export interface Demo3DInstance {
  sliderDefs: SliderDef[]
  /**
   * 滑块拖动入口：绝对值直接赋给目标 mesh 对应属性。
   * rotation 输入角度（内部转弧度），translation 输入世界坐标，scale 输入倍率。
   */
  applySlider(type: 'rotation' | 'translation' | 'scale', targetId: string, axis: 'x' | 'y' | 'z', value: number): void
  dispose(): void
}

/** three.js 的位置类 rx/rz 方法均接收分离分量，统一展开三元组 */
const vec = (v: Vec3): [number, number, number] => [v[0], v[1], v[2]]

// ---- 几何体工厂（args 逐位取：合法正数用声明值，否则取默认） ----

function buildGeometry(mesh: SceneMesh): BufferGeometry {
  const a = mesh.args
  const pick = (...defaults: number[]): number[] =>
    defaults.map((d, i) => (Number.isFinite(a[i]) && (a[i] as number) > 0 ? (a[i] as number) : d))
  switch (mesh.type) {
    case 'Sphere':
      return new SphereGeometry(...vec(pick(1, 32, 16) as [number, number, number]))
    case 'Cylinder':
      return new CylinderGeometry(...(pick(0.5, 0.5, 1, 32) as [number, number, number, number]))
    case 'Cone':
      return new ConeGeometry(...(pick(0.5, 1, 32) as [number, number, number]))
    case 'Torus':
      return new TorusGeometry(...(pick(1, 0.4, 16, 100) as [number, number, number, number]))
    case 'TorusKnot':
      return new TorusKnotGeometry(...(pick(1, 0.4, 128, 16) as [number, number, number, number]))
    case 'Plane':
      return new PlaneGeometry(...(pick(2, 2) as [number, number]))
    case 'Icosahedron':
      return new IcosahedronGeometry(...(pick(1, 0) as [number, number]))
    case 'Box':
    default:
      return new BoxGeometry(...(pick(1, 1, 1) as [number, number, number]))
  }
}

// ---- 材质工厂 ----

function buildMaterial(m: SceneMaterial): Material {
  const base = { color: m.color, opacity: m.opacity, transparent: m.opacity < 1 }
  switch (m.type) {
    case 'MeshBasicMaterial':
      return new MeshBasicMaterial(base)
    case 'MeshLambertMaterial':
      return new MeshLambertMaterial({ ...base, emissive: m.emissive })
    case 'MeshPhongMaterial':
      return new MeshPhongMaterial({ ...base, emissive: m.emissive })
    case 'MeshPhysicalMaterial':
      return new MeshPhysicalMaterial({ ...base, emissive: m.emissive, roughness: m.roughness, metalness: m.metalness })
    case 'MeshNormalMaterial':
      return new MeshNormalMaterial()
    case 'MeshStandardMaterial':
    default:
      return new MeshStandardMaterial({ ...base, emissive: m.emissive, roughness: m.roughness, metalness: m.metalness })
  }
}

// ---- 灯光工厂（指向型：position + target 两点定朝向；castShadow 仅声明） ----

function buildNodesOfLight(l: SceneLight): Object3D[] {
  switch (l.type) {
    case 'AmbientLight':
      return [new AmbientLight(l.color, l.intensity)]
    case 'HemisphereLight':
      return [new HemisphereLight(l.color, '#1e293b', l.intensity)]
    case 'PointLight': {
      const light = new PointLight(l.color, l.intensity)
      light.position.set(...vec(l.position))
      return [light]
    }
    case 'SpotLight': {
      const light = new SpotLight(l.color, l.intensity)
      light.position.set(...vec(l.position))
      light.target.position.set(...vec(l.target))
      return [light, light.target]
    }
    case 'DirectionalLight':
    default: {
      const light = new DirectionalLight(l.color, l.intensity)
      light.position.set(...vec(l.position))
      light.target.position.set(...vec(l.target))
      return [light, light.target]
    }
  }
}

/** 各类型滑块的系统固定值域 */
function rangeOf(type: 'rotation' | 'translation' | 'scale'): { min: number; max: number; step: number } {
  return { ...CONTROL_RANGES[type] }
}

/**
 * 构建完整可交互三维场景。
 * @param container 渲染容器（renderer canvas 挂其内）
 * @param def 修剪后的场景定义
 */
export function buildDemo3D(container: HTMLElement, def: Demo3DSceneDef): Demo3DInstance {
  // ---- renderer / scene / 相机 ----
  const renderer = new WebGLRenderer({ antialias: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.domElement.style.width = '100%'
  renderer.domElement.style.height = '100%'
  renderer.domElement.style.display = 'block'
  container.appendChild(renderer.domElement)

  const scene = new Scene()
  scene.background = new Color(def.scene.background)

  const camera: PerspectiveCamera | OrthographicCamera =
    def.camera.type === 'OrthographicCamera'
      ? new OrthographicCamera(-1, 1, 1, -1, 0.1, 5000)
      : new PerspectiveCamera(def.camera.fov, 1, 0.1, 5000)
  camera.position.set(...vec(def.camera.position))
  camera.lookAt(new Vector3(...vec(def.camera.lookAt)))

  /** 依容器尺寸刷新 renderer 与相机投影 */
  const resize = (): void => {
    const w = Math.max(container.clientWidth, 1)
    const h = Math.max(container.clientHeight, 1)
    const aspect = w / h
    renderer.setSize(w, h)
    if (camera instanceof PerspectiveCamera) {
      camera.aspect = aspect
    }
    // 正交相机视锥：以水平 distance 视锥（契约中正交相机不提供 fov）
    else {
      const size = camera.position.length() || 10
      camera.left = -size * aspect
      camera.right = size * aspect
      camera.top = size
      camera.bottom = -size
    }
    camera.updateProjectionMatrix()
  }

  // ---- 场景全局：坐标轴 + 地面网格辅助 ----
  if (def.scene.axesHelper) {
    scene.add(new AxesHelper(10))
    scene.add(new GridHelper(20, 20, 0x334155, 0x1e293b))
  }

  // ---- 灯光（指向型灯光的 target 对象需入场景才生效） ----
  for (const l of def.lights) {
    for (const node of buildNodesOfLight(l)) {
      scene.add(node)
    }
  }

  // ---- 几何体 + 材质（两阶段：先创建全部 mesh，再按 parentId 组装场景树） ----
  const materials = new Map<string, Material>()
  for (const m of def.materials) {
    materials.set(m.id, buildMaterial(m))
  }
  const geometries: BufferGeometry[] = []
  const meshById = new Map<string, Mesh>()
  for (const mesh of def.meshes) {
    const geo = buildGeometry(mesh)
    geometries.push(geo)
    const obj = new Mesh(geo, materials.get(mesh.materialId) ?? materials.get(DEFAULT_MATERIAL_ID))
    obj.position.set(...vec(mesh.position))
    obj.rotation.set(...vec(mesh.rotation))
    obj.scale.set(...vec(mesh.scale))
    meshById.set(mesh.id, obj)
  }
  // 挂树：parentId 指向父 mesh（变换级联带动子树），null 挂场景根
  for (const mesh of def.meshes) {
    const obj = meshById.get(mesh.id)
    const parent = (mesh.parentId ? meshById.get(mesh.parentId) : null) ?? scene
    parent.add(obj as Mesh)
  }

  // ---- 控制器 ----
  // orbit：OrbitControls（拖拽环绕相机 + 滚轮缩放距离）；autoRotate 仅声明不实现。
  let orbitControl: OrbitControls | undefined
  if (def.controls.some((c) => c.type === 'orbit')) {
    orbitControl = new OrbitControls(camera, renderer.domElement)
  }

  // 模型变换滑块：绝对值赋值，零状态；初值取该 mesh 当前变换换算值
  const sliderDefs: SliderDef[] = []
  for (const c of def.controls) {
    if (c.type === 'orbit') continue
    const mesh = meshById.get(c.targetId)
    if (!mesh) continue
    switch (c.type) {
      case 'rotation':
        sliderDefs.push({
          control: c,
          ...rangeOf('rotation'),
          initial: MathUtils.clamp(mesh.rotation[c.axis] * (180 / Math.PI), CONTROL_RANGES.rotation.min, CONTROL_RANGES.rotation.max),
        })
        break
      case 'translation':
        sliderDefs.push({
          control: c,
          ...rangeOf('translation'),
          initial: MathUtils.clamp(mesh.position[c.axis], CONTROL_RANGES.translation.min, CONTROL_RANGES.translation.max),
        })
        break
      case 'scale':
        sliderDefs.push({ control: c, ...rangeOf('scale'), initial: 1 })
        break
      default:
        break
    }
  }

  const applySlider = (
    type: 'rotation' | 'translation' | 'scale',
    targetId: string,
    axis: 'x' | 'y' | 'z',
    value: number,
  ): void => {
    const mesh = meshById.get(targetId)
    if (!mesh) return
    switch (type) {
      case 'rotation':
        mesh.rotation[axis] = MathUtils.degToRad(value)
        break
      case 'translation':
        mesh.position[axis] = value
        break
      case 'scale':
        mesh.scale[axis] = value
        break
    }
  }

  // ---- 动画循环 + resize 监听 ----
  renderer.setAnimationLoop(() => {
    if (orbitControl) orbitControl.update()
    renderer.render(scene, camera)
  })
  const observer = new ResizeObserver(resize)
  observer.observe(container)
  resize()

  // ---- 全量释放 ----
  const dispose = (): void => {
    observer.disconnect()
    renderer.setAnimationLoop(null)
    for (const g of geometries) g.dispose()
    for (const m of materials.values()) m.dispose()
    orbitControl?.dispose()
    renderer.dispose()
    renderer.domElement.remove()
  }

  return { sliderDefs, applySlider, dispose }
}
