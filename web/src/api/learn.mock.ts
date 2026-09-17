import type { DemoContent, Demo3DContent } from './learn'

/**
 * learn mock：学习页的进度/演示代码的内存态 + demo 兜底数据。
 *
 * slide 的 content/steps 与 quiz 的题目已改为从真实接口获取（见 learn.ts getCourseDetail，
 * quiz 题目来自后端 question 表）；问答会话已走真实接口（api/discussion.ts
 * listConversation）；demo 后端尚未生成真实内容，故仅保留 demo 兜底。
 */

// 内存态
const progressByCourse = new Map<string, string>()
const demoCodeBySection = new Map<string, string>()

// ---- demo 兜底数据（后端尚未生成真实内容）----
// demo_basic / demo_3d 均可预览；demo_function 需预注入绘图库，本期未实现。

export const DEMO_CONTENT: DemoContent = {
  code: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <style>
    body { font-family: system-ui, sans-serif; display: flex; gap: 16px; align-items: center; justify-content: center; height: 100vh; margin: 0; background: #f8fafc; }
    .cell { width: 60px; height: 60px; display: flex; align-items: center; justify-content: center; border: 2px solid #0f766e; border-radius: 8px; font-size: 22px; color: #0f172a; }
    .label { position: absolute; top: -22px; left: 0; right: 0; text-align: center; font-size: 12px; color: #64748b; }
    button { margin-top: 12px; }
  </style>
</head>
<body>
  <div style="position:relative">
    <div id="box" style="display:flex; gap:8px; position:relative"></div>
    <button onclick="reverse()">反转数组</button>
  </div>
  <script>
    const values = [1, 2, 3, 4, 5];
    function render() {
      const box = document.getElementById('box');
      box.innerHTML = '';
      values.forEach((v, i) => {
        const cell = document.createElement('div');
        cell.className = 'cell';
        cell.innerHTML = v + '<span class="label">arr[' + i + ']</span>';
        cell.style.position = 'relative';
        box.appendChild(cell);
      });
    }
    function reverse() { values.reverse(); render(); }
    render();
  </script>
</body>
</html>`,
}

/** demo_3d 兜底：水分子 V 形示意（与后端 few-shot 示例一致，氢/键以 parentId 挂氧原子组成场景树）。 */
export const DEMO3D_CONTENT: Demo3DContent = {
  scene: { background: '#0f172a', axesHelper: false },
  geometries: [
    { id: 'atom-o', type: 'Sphere', args: [1.2, 32, 16], position: [0, 0, 0], rotation: [0, 0, 0], scale: 1, materialId: 'mat-o' },
    { id: 'atom-h1', type: 'Sphere', args: [0.6, 32, 16], position: [1.7, -0.9, 0], rotation: [0, 0, 0], scale: 1, materialId: 'mat-h', parentId: 'atom-o' },
    { id: 'atom-h2', type: 'Sphere', args: [0.6, 32, 16], position: [-1.7, -0.9, 0], rotation: [0, 0, 0], scale: 1, materialId: 'mat-h', parentId: 'atom-o' },
    { id: 'bond-1', type: 'Cylinder', args: [0.15, 0.15, 1.8, 16], position: [0.85, -0.45, 0], rotation: [0, 0, 1.1], scale: 1, materialId: 'mat-bond', parentId: 'atom-o' },
    { id: 'bond-2', type: 'Cylinder', args: [0.15, 0.15, 1.8, 16], position: [-0.85, -0.45, 0], rotation: [0, 0, -1.1], scale: 1, materialId: 'mat-bond', parentId: 'atom-o' },
  ],
  materials: [
    { id: 'mat-o', type: 'MeshStandardMaterial', color: '#f43f5e', roughness: 0.35, metalness: 0.1 },
    { id: 'mat-h', type: 'MeshStandardMaterial', color: '#38bdf8', roughness: 0.35, metalness: 0.1 },
    { id: 'mat-bond', type: 'MeshStandardMaterial', color: '#e2e8f0', roughness: 0.6, metalness: 0 },
  ],
  lights: [
    { type: 'AmbientLight', color: '#ffffff', intensity: 0.45 },
    { type: 'DirectionalLight', color: '#ffffff', intensity: 1.6, position: [6, 8, 4], target: [0, 0, 0] },
  ],
  cameras: [{ type: 'PerspectiveCamera', position: [0, 2, 8], fov: 45, lookAt: [0, -0.3, 0] }],
  controls: [
    { type: 'orbit', title: '环绕视角', autoRotate: false },
    { type: 'rotation', title: '氧原子自转', targetId: 'atom-o', axis: 'y' },
  ],
}

// ---- 变更 ----

export function setProgress(courseId: string, status: string): void {
  progressByCourse.set(courseId, status)
}

export function setDemoCode(sectionId: string, code: string): void {
  demoCodeBySection.set(sectionId, code)
}
