# ModelScope API 文档

本文档描述了 `api-inference.modelscope.cn` 提供的图像生成和编辑 API。

## 基础URL

**`https://api-inference.modelscope.cn`**

---

## 核心概念：异步调用

ModelScope 的图像生成/编辑接口是 **异步** 的。这意味着调用不会立即返回结果，而是遵循以下流程：

1.  **提交任务**: 客户端向 `/v1/images/generations` 发送一个 `POST` 请求来启动一个任务。
2.  **获取 Task ID**: 如果请求成功，API 会立即返回一个 `task_id`。
3.  **轮询状态**: 客户端需要使用这个 `task_id`，通过向 `/v1/tasks/{task_id}` 发送 `GET` 请求来定期查询任务的执行状态。
4.  **获取结果**: 当任务状态变为 `SUCCEED` 时，响应中会包含最终生成的图片 URL。

---

## 1. 生成/编辑图片

文生图和图生图使用相同的端点，仅通过 `model` 和 `image_url` 参数来区分。

- **URL**: `/v1/images/generations`
- **方法**: `POST`

### 关键请求头 (Headers)

| Header | 值 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `Authorization` | `Bearer <YOUR_API_KEY>` | 是 | 用于身份认证。 |
| `Content-Type` | `application/json` | 是 | 指定请求体格式。 |
| `X-ModelScope-Async-Mode` | `true` | **是** | **必须携带此Header以启用异步模式，否则请求会失败。** |

### JSON 请求体示例

#### 文生图 (Text-to-Image)
```json
{
  "model": "Qwen/Qwen-Image",
  "prompt": "A golden cat",
  "size": "2048x2048"
}
```

#### 图生图 (Image-to-Image)
```json
{
  "model": "Qwen/Qwen-Image-Edit",
  "prompt": "turn the girl's hair blue",
  "image_url": "https://resources.modelscope.cn/aigc/image_edit.png",
  "size": "2048x2048"
}
```

### 参数详解

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `model` | string | 是 | 指定使用的模型。例如 `Qwen/Qwen-Image` (文生图) 或 `Qwen/Qwen-Image-Edit` (图生图)。 |
| `prompt` | string | 是 | 文本描述。 |
| `image_url` | string | 否 | **图生图时必须**。提供需要编辑的图片的 URL。 |
| `size` | string | 否 | 输出图片的尺寸，范围为 64-2048。例如：`1024x1024`。 |

---

## 2. 轮询任务状态

- **URL**: `/v1/tasks/{task_id}`
- **方法**: `GET`

### 关键请求头 (Headers)

| Header | 值 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `Authorization` | `Bearer <YOUR_API_KEY>` | 是 | 用于身份认证。 |
| `X-ModelScope-Task-Type` | `image_generation` | **是** | **必须携带此Header以指定任务类型，否则无法正确查询状态。** |

### 响应详解

#### 任务处理中 (PROCESSING)
```json
{
  "request_id": "some-request-id",
  "task_id": "your-task-id",
  "task_status": "PROCESSING",
  "outputs": {}
}
```

#### 任务成功 (SUCCEED)
```json
{
  "task_status": "SUCCEED",
  "output_images": [
      "https://some-url-to-your-image.jpg"
  ],
  "request_id": "some-request-id",
  "task_id": "your-task-id"
}
```

#### 任务失败 (FAILED)
当任务失败时，响应中会包含一个 `errors` 对象，其中有详细的失败原因。
```json
{
  "errors": {
    "code": 422,
    "message": "Output data may contain inappropriate content."
  },
  "request_id": "some-request-id",
  "task_status": "FAILED"
}
```
常见的失败原因包括内容安全审核失败、参数错误等。

---

## 3. 当前轮询策略

在我们的后端实现中，采用了以下轮询策略：
- **轮询间隔**: 每 **5 秒** 查询一次任务状态。
- **超时时间**: 如果 **5 分钟** (300 秒) 后任务仍未完成，则视为超时。