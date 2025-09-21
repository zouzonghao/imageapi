# NodeImage API 文档

本文档描述了 NodeImage 图床提供的图片上传 API。

## 基础URL

**`https://api.nodeimage.com`**

---

## 1. 上传图片

此端点用于上传图片文件并获取可访问的 URL。

- **URL**: `/api/upload`
- **方法**: `POST`
- **请求格式**: `multipart/form-data`
- **认证**: `X-API-Key: <YOUR_API_KEY>`

### 表单数据 (Form Data)

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `image` | file | 是 | 要上传的图片文件。 |

### 成功响应

  **JSON 对象**: 包含上传成功信息和多种格式的图片链接。
    - **Content-Type**: `application/json`
    ```json
    {
      "success": true,
      "message": "Image uploaded successfully",
      "image_id": "u5xIj6oZMkNwv393EucWhod0XFR51rlk",
      "filename": "u5xIj6oZMkNwv393EucWhod0XFR51rlk.jpeg",
      "size": 86961,
      "links": {
        "direct": "https://cdn.nodeimage.com/i/u5xIj6oZMkNwv393EucWhod0XFR51rlk.jpeg",
        "html": "<img src=\"https://cdn.nodeimage.com/i/u5xIj6oZMkNwv393EucWhod0XFR51rlk.jpeg\" alt=\"image\">",
        "markdown": "![image](https://cdn.nodeimage.com/i/u5xIj6oZMkNwv393EucWhod0XFR51rlk.jpeg)",
        "bbcode": "[img]https://cdn.nodeimage.com/i/u5xIj6oZMkNwv393EucWhod0XFR51rlk.jpeg[/img]"
      }
    }
    ```

### cURL 示例

```bash
curl -X POST "https://api.nodeimage.com/api/upload" \
  -H "X-API-Key: <YOUR_API_KEY>" \
  -F "image=@<PATH_TO_YOUR_IMAGE.jpeg>"
```

## 2. 删除图片

此端点用于删除已上传的图片。

- **URL**: `/api/v1/delete/<IMAGE_ID>`
- **方法**: `DELETE`
- **认证**: `X-API-Key: <YOUR_API_KEY>`

### URL 参数 (URL Parameters)

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `IMAGE_ID` | string | 是 | 要删除的图片的 ID。 |

### 成功响应

  **JSON 对象**: 包含删除成功的信息。
    - **Content-Type**: `application/json`
    ```json
    {
      "success": true,
      "message": "删除成功"
    }
    ```

### cURL 示例

```bash
curl -X DELETE "https://api.nodeimage.com/api/v1/delete/<IMAGE_ID>" \
  -H "X-API-Key: <YOUR_API_KEY>"
```