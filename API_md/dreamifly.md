# Dreamifly API 文档

本文档描述了 `dreamifly.com` 提供的图像生成和提示词优化 API。

## 基础URL

**`https://dreamifly.com`**

---

## 1. 生成图片

此端点根据提供的参数生成图片。

- **URL**: `/api/generate`
- **方法**: `POST`
- **请求格式**: `application/json`

### JSON 请求体

```json
{
  "prompt": "a beautiful landscape",
  "width": 1024,
  "height": 1024,
  "steps": 50,
  "seed": 12345,
  "batch_size": 1,
  "model": "stable-diffusion",
  "images": [],
  "denoise": 0.7
}
```

### 参数详解

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `prompt` | string | 是 | 用于生成图片的文本描述。 |
| `width` | integer | 否 | 生成图片的宽度。范围：64-1920。 |
| `height` | integer | 否 | 生成图片的高度。范围：64-1920。 |
| `steps` | integer | 是 | 图像生成的迭代步数。 |
| `seed` | integer | 是 | 随机种子，用于生成可复现的结果。 |
| `batch_size` | integer | 是 | 一次生成的图片数量，通常为 `1`。 |
| `model` | string | 是 | 指定使用的生成模型。可用模型包括：`Flux-Kontext`, `Qwen-Image-Edit`, `Wai-SDXL-V150`, `Flux-Krea`, `HiDream-full-fp8`, `Qwen-Image`。 |
| `images` | array of strings | 否 | Base64 编码的图片字符串数组。用于图生图模式。**注意：** 只有 `Flux-Kontext` 和 `Qwen-Image-Edit` 模型接受此字段有值，其他模型请提供空数组 `[]`。 |
| `denoise` | float | 否 | 降噪强度，通常在 `0.5` 到 `1.0` 之间。 |

### 成功响应


  **JSON 对象**: 包含一个 Base64 编码的 Data URL。
    - **Content-Type**: `application/json`
    ```json
    {
      "imageUrl": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg..."
    }
    ```


### cURL 示例

```bash
curl -X POST https://dreamifly.com/api/generate \
  -H "Content-Type: application/json" \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36" \
  -H "Referer: https://dreamifly.com/zh" \
  -d '{
    "prompt": "a beautiful landscape",
    "model": "stable-diffusion",
    "steps": 50,
    "seed": 12345,
    "width": 1024,
    "height": 1024,
    "batch_size": 1,
    "images": [],
    "denoise": 0.7
  }'
```

---

## 2. 优化提示词

此端点用于将输入的提示词优化，使其更适合图像生成模型。

- **URL**: `/api/optimize-prompt`
- **方法**: `POST`
- **请求格式**: `application/json`

### JSON 请求体

```json
{
  "prompt": "a cat"
}
```

### 成功响应

- **Content-Type**: `application/json`
- **响应体**: 返回一个包含优化后提示词的JSON对象。

```json
{
  "success": true,
  "originalPrompt": "a cat",
  "optimizedPrompt": "a detailed, photorealistic image of a fluffy cat sitting on a sunlit windowsill"
}
```

### cURL 示例

```bash
curl -X POST https://dreamifly.com/api/optimize-prompt \
  -H "Content-Type: application/json" \
  -H "Accept: */*" \
  -H "Origin: https://dreamifly.com" \
  -H "Referer: https://dreamifly.com/zh" \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36" \
  -d '{
    "prompt": "a cat"
  }'