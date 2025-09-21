# Fal.ai API 文档

本文档描述了 `fal.run` 提供的图像编辑 API。

## 基础URL

**`https://fal.run`**

---

## 1. 图像编辑

此端点根据提供的参数编辑图像。

- **URL**: `/fal-ai/bytedance/seedream/v4/edit`
- **方法**: `POST`
- **请求格式**: `application/json`
- **认证**: `Authorization: Key $FAL_KEY`

### JSON 请求体

```json
{
  "prompt": "Dress the model in the clothes and hat. Add a cat to the scene and change the background to a Victorian era building.",
  "image_size": {
    "height": 2160,
    "width": 3840
  },
  "num_images": 1,
  "max_images": 1,
  "enable_safety_checker": true,
  "image_urls": [
    "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_1.png",
    "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_2.png",
    "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_3.png",
    "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_4.png"
  ]
}
```

### 参数详解

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `prompt` | string | 是 | 用于编辑图像的文本提示。 |
| `image_urls` | array of strings | 是 | 用于编辑的输入图像的 URL 列表。 |
| `image_size` | object | 否 | 生成图像的尺寸。宽度和高度必须在 1024 到 4096 之间。默认值：`{"height":2048,"width":2048}`。 |
| `num_images` | integer | 否 | 要运行的独立模型生成次数。默认值：`1`。 |
| `max_images` | integer | 否 | 如果设置为大于1的数字，则启用多图像生成。默认值：`1`。 |
| `seed` | integer | 否 | 控制图像生成随机性的随机种子。 |
| `sync_mode` | boolean | 否 | 如果设置为 true，函数将等待图像生成并上传后再返回响应。默认值：`false`。 |
| `enable_safety_checker` | boolean | 否 | 如果设置为 true，将启用安全检查器。默认值：`true`。 |

### 成功响应

  **JSON 对象**: 包含生成的图像信息和使用的种子。
    - **Content-Type**: `application/json`
    ```json
    {
      "images": [
        {
          "url": "https://storage.googleapis.com/falserverless/example_outputs/seedream4_edit_output.png"
        }
      ],
      "seed": 746406749
    }
    ```

### cURL 示例

```bash
curl --request POST \
  --url https://fal.run/fal-ai/bytedance/seedream/v4/edit \
  --header "Authorization: Key $FAL_KEY" \
  --header "Content-Type: application/json" \
  --data '{
     "prompt": "Dress the model in the clothes and hat. Add a cat to the scene and change the background to a Victorian era building.",
     "image_urls": [
       "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_1.png",
       "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_2.png",
       "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_3.png",
       "https://storage.googleapis.com/falserverless/example_inputs/seedream4_edit_input_4.png"
     ]
   }'