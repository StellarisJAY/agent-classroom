import type { ChatMessage, DemoContent } from './learn'

/**
 * learn mock：学习页的会话/进度/演示代码的内存态 + demo 兜底数据。
 *
 * slide 的 content/steps 与 quiz 的题目已改为从真实接口获取（见 learn.ts getCourseDetail，
 * quiz 题目来自后端 question 表）；demo 后端尚未生成真实内容，故仅保留 demo 兜底。
 */

// 内存态
const messagesByCourse = new Map<string, ChatMessage[]>()
const progressByCourse = new Map<string, string>()
const demoCodeBySection = new Map<string, string>()

function nowIso(): string {
  return new Date().toISOString()
}

let msgSeq = 0

function seedMessages(courseId: string): ChatMessage[] {
  const existing = messagesByCourse.get(courseId)
  if (existing) return existing
  const list: ChatMessage[] = [
    {
      id: `m${++msgSeq}`,
      role: 'assistant',
      content:
        '你好，我是这节课程的智能老师。数组是一段连续内存里存储的同类型元素集合，有任何疑问都可以直接问我。',
      section_id: null,
      created_at: nowIso(),
    },
  ]
  messagesByCourse.set(courseId, list)
  return list
}

// ---- demo 兜底数据（后端尚未生成真实内容）----

export const DEMO_CONTENT: DemoContent = {
  subtype: 'basic',
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

// ---- 消息 ----

export function listMessages(courseId: string): ChatMessage[] {
  return seedMessages(courseId)
}

export function buildAssistantReply(question: string, sectionId: string | null): string {
  const section = sectionId === 'sec-quiz'
  if (section) {
    return (
      '我提示一下思路，但不会直接给出答案：' +
      '注意区分数组声明与初始化，以及下标访问的起点。你可以结合前面的讲解再想想，' +
      '如果仍有疑问，换个角度问我「为什么会这样」也可以。'
    )
  }
  const trimmed = question.trim()
  if (trimmed.includes('为什么') || trimmed.includes('区别')) {
    return '这是一个很好的问题。数组把同类型元素放进连续内存，正因如此才能按下标做 O(1) 随机访问。需要的话，我可以进一步解释内存布局与访问原理。'
  }
  return '根据这节课的内容：数组是连续内存里的同类型元素集合，声明后长度固定，按下标（从 0 起）访问。你可以点击「下一步」回顾讲解步骤，或上传疑问我继续解答。'
}

// ---- 变更 ----

export function setProgress(courseId: string, status: string): void {
  progressByCourse.set(courseId, status)
}

export function setDemoCode(sectionId: string, code: string): void {
  demoCodeBySection.set(sectionId, code)
}
