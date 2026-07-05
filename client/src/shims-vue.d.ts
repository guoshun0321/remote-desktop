// 让 TypeScript 识别 .vue 文件(本骨架暂用最简 shim;vue-tsc 真正做类型检查)。
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}
