# Pollinations.ai API 文档

本文档描述了 `image.pollinations.ai` 提供的图像生成 API。

## 基础URL

**`https://image.pollinations.ai`**

---

## 1. 生成图片

此端点根据提供的文本描述和参数生成一张图像。

- **URL**: `/prompt/{prompt}`
- **方法**: `GET`

### 请求头 (Headers)

| 参数 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `Authorization` | string | 否 | 用于身份验证的 Bearer Token。格式: `Bearer {your_api_key}` |

### URL 参数

| 参数 | 类型 | 是否必须 | 描述 | 默认值 |
| :--- | :--- | :--- | :--- | :--- |
| `prompt` | string | 是 | 图像的文本描述（需要 URL 编码）。**注意：此参数是 URL 路径的一部分，而不是查询参数。** | |
| `model` | string | 否 | 用于生成的模型。 | `flux` |
| `seed` | integer | 否 | 用于生成可复现结果的种子。 | |
| `width` | integer | 否 | 生成图像的宽度（像素）。 | 1024 |
| `height` | integer | 否 | 生成图像的高度（像素）。 | 1024 |
| `image` | string | 否 | 用于图生图/编辑的输入图像 URL (`kontext`模型)。 | |
| `nologo` | boolean | 否 | 设置为 `true` 以禁用 Pollinations 徽标。 | `false` |
| `private` | boolean | 否 | 设置为 `true` 以防止图像出现在公共 Feed 中。 | `false` |
| `enhance` | boolean | 否 | 设置为 `true` 以使用 LLM 增强提示。 | `false` |
| `safe` | boolean | 否 | 设置为 `true` 以启用严格的 NSFW 过滤。 | `false` |

### 成功响应

- **Content-Type**: `image/jpeg` (或其他图像格式)
- **响应体**: 直接返回图像文件。

### cURL 示例

#### 文生图
注意，prompt需要URL编码
```bash

# 带参数示例
curl -o sunset_large.jpg -H "Authorization: Bearer {your_api_key}" "https://image.pollinations.ai/prompt/A%20beautiful%20sunset%20over%20the%20ocean?width=1280&height=720&seed=42&model=flux&nologo=true"
```

#### 图生图 (使用 kontext 模型)
注意，prompt需要URL编码，同时image的值也需要URL编码
```bash
curl -o logo_cake.png -H "Authorization: Bearer {your_api_key}" "https://image.pollinations.ai/prompt/2D%2C%20anime?model=kontext&nologo=true&image=https%3A%2F%2Fimg.8666999.xyz%2Fi%2F2025%2F09%2F21%2Fxrw6tt.webp&quality=high&width=720&height=1024"
```
