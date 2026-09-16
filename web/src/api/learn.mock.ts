import type { DemoContent } from './learn'

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
// 仅 demo_basic 目前可运行预览；demo_3d / demo_function 需预注入库，本期未实现。

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

// ---- 变更 ----

export function setProgress(courseId: string, status: string): void {
  progressByCourse.set(courseId, status)
}

export function setDemoCode(sectionId: string, code: string): void {
  demoCodeBySection.set(sectionId, code)
}
