import type { ChatMessage, DemoContent, Question } from './learn'

/**
 * learn mock：学习页的会话/进度/演示代码的内存态 + quiz/demo 兜底数据。
 *
 * slide 环节的 content / steps 已改为从真实接口获取（见 learn.ts getCourseDetail），
 * 此处不再提供 slide 示例；quiz / demo 后端尚未生成真实内容，故保留示例供前端兜底。
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

// ---- quiz / demo 兜底数据（后端尚未生成真实内容）----

export const QUIZ_QUESTIONS: Question[] = [
  {
    id: 'q1',
    position: 1,
    type: 'single',
    stem: '以下对数组的描述，哪一项是正确的？',
    options: [
      '数组可以存放不同类型的元素',
      '数组是一段连续内存里的同类型元素集合',
      '数组长度可以在运行时随意改变',
      '数组下标从 1 开始计数',
    ],
    answers: [1],
    explanations: [
      '错误：同一数组内元素类型一致。',
      '正确：数组是连续内存上的同类型元素集合。',
      '错误：数组长度在声明后固定。',
      '错误：数组下标从 0 开始。',
    ],
  },
  {
    id: 'q2',
    position: 2,
    type: 'multiple',
    stem: '关于数组的初始化与访问，下列说法正确的有？',
    options: [
      '使用大括号 {1,2,3,4,5} 进行初始化',
      '下标从 0 开始，因此首元素是 arr[0]',
      '可以对数组进行整体赋值后再改变长度',
      '长度在声明时确定，之后保持不变',
    ],
    answers: [0, 1, 3],
    explanations: [
      '正确：初始化时以大括号给出各元素初值。',
      '正确：数组下标从 0 起，首元素为 arr[0]。',
      '错误：数组不支持运行时改变长度。',
      '正确：声明后长度固定。',
    ],
  },
]

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
