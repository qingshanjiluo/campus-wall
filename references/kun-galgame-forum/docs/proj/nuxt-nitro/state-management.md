# 状态管理

## 概述

项目使用 Pinia 作为状态管理方案，配合 `pinia-plugin-persistedstate` 实现状态持久化。Store 按职责分为三类：

- **持久化 Store** (`app/store/modules/`) - 跨会话保存的用户偏好和编辑草稿
- **临时 Store** (`app/store/temp/`) - 仅会话级别的 UI 状态
- **类型定义** (`app/store/types/`) - Store 状态的 TypeScript 接口

## Store 定义模式

### 持久化 Store

使用 `usePersist*` 前缀，启用 `persist` 选项：

```typescript
// app/store/modules/edit/galgame.ts
export const usePersistEditGalgameStore = defineStore(
  'KUNGalgameEditGalgame',  // 唯一标识符
  () => {
    // 状态声明
    const vndbId = ref<GalgameStorePersist['vndbId']>('')
    const name = reactive<GalgameStorePersist['name']>({
      'en-us': '',
      'ja-jp': '',
      'zh-cn': '',
      'zh-tw': ''
    })

    // 重置方法
    const resetEditGalgameStore = () => {
      vndbId.value = ''
      resetReactiveState(name, createEmptyLocaleMap())
    }

    return { vndbId, name, resetEditGalgameStore }
  },
  {
    persist: {
      storage: piniaPluginPersistedstate.localStorage()
    }
  }
)
```

### 临时 Store

使用 `useTemp*` 前缀，不启用持久化：

```typescript
// app/store/temp/search.ts
export const useTempSearchStore = defineStore('tempSearch', () => {
  const keywords = ref('')
  return { keywords }
})
```

### 组件级 Store

用于跨组件通信的 UI 状态：

```typescript
// app/store/temp/components/message.ts
export const useComponentMessageStore = defineStore('componentMessage', () => {
  const showAlert = ref(false)
  const alertTitle = ref('')
  const alertMsg = ref('')
  return { showAlert, alertTitle, alertMsg }
})
```

## 核心 Store 列表

| Store 名称 | 类型 | 职责 |
|------------|------|------|
| `usePersistUserStore` | 持久化 | 用户身份信息（id, name, role, avatar） |
| `usePersistEditTopicStore` | 持久化 | 话题编辑草稿 |
| `usePersistEditGalgameStore` | 持久化 | Galgame 编辑草稿 |
| `usePersistEditGalgameRatingStore` | 持久化 | 评分编辑草稿 |
| `usePersistKUNGalgameReplyStore` | 持久化 | 回复编辑草稿 |
| `usePersistKUNGalgameTopicStore` | 持久化 | 话题列表布局偏好 |
| `usePersistCategoryStore` | 持久化 | 分类浏览偏好 |
| `usePersistSettingsStore` | 持久化 | 用户设置（透明度、字体、内容限制、背景等） |
| `usePersistSearchStore` | 持久化 | 搜索偏好 |
| `usePersistKUNGalgameAdvancedFilterStore` | 持久化 | 高级筛选条件 |
| `useTempEditStore` | 临时 | 编辑临时状态 |
| `useTempGalgamePRStore` | 临时 | PR 临时数据 |
| `useTempGalgameResourceStore` | 临时 | 资源临时数据 |
| `useTempReplyStore` | 临时 | 回复临时状态 |
| `useTempSearchStore` | 临时 | 搜索临时状态 |
| `useComponentMessageStore` | 临时 | 消息弹窗状态 |

## 工具函数

`app/store/index.ts` 提供以下工具：

```typescript
// 创建空多语言映射
createEmptyLocaleMap()
// → { 'en-us': '', 'ja-jp': '', 'zh-cn': '', 'zh-tw': '' }

// 重置 reactive 对象状态
resetReactiveState(target, defaults)

// 全局重置所有 Store（登出时调用）
kungalgameStoreReset()
```

## 持久化配置

通过 `nuxt.config.ts` 配置：

```typescript
piniaPluginPersistedstate: {
  cookieOptions: {
    maxAge: 60 * 60 * 24 * 7,  // 7 天
    sameSite: 'strict'
  }
}
```

Store 可选择 `localStorage` 或 `cookie` 作为存储后端。

## 状态管理最佳实践

1. **SSR 安全状态**: 在 Composable 中使用 `useState()` 而非 `ref()`，确保 SSR 水合正确
2. **草稿保存**: 编辑器相关状态使用持久化 Store，防止意外丢失
3. **UI 状态**: 临时的交互状态使用 `useTemp*` Store
4. **登出清理**: `kungalgameStoreReset()` 统一重置所有 Store
5. **类型安全**: 每个 Store 都有对应的类型定义在 `app/store/types/`
