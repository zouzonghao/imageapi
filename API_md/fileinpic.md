# fileinpic API 文档

本文档描述了 fileinpic 图床提供的文件上传、下载和删除 API。

## 基础URL

**`https://file.oneonezero.dpdns.org`**

---

## 1. 上传文件

此端点用于上传文件并获取可访问的 URL。

- **URL**: `/api/v1/files/upload`
- **方法**: `POST`
- **认证**: `X-API-KEY: <YOUR_API_KEY>`

### 请求头 (Headers)

| 参数名 | 是否必须 | 描述 |
| :--- | :--- | :--- |
| `X-API-KEY` | 是 | 您的 API 密钥。 |
| `Content-Disposition` | 是 | 指定文件名，格式为 `attachment; filename="<FILENAME>"`。 |

### 请求体 (Body)

- **格式**: `binary`
- **描述**: 使用 `--data-binary` 发送文件内容。

### 成功响应

  **JSON 对象**: 包含上传成功状态和文件的下载链接。
    - **Content-Type**: `application/json`
    ```json
    {
      "ok": true,
      "url": "/api/v1/files/public/download/8"
    }
    ```

### cURL 示例

```bash
curl -X POST \
  -H "X-API-KEY: <YOUR_API_KEY>" \
  -H "Content-Disposition: attachment; filename=\"test.txt\"" \
  --data-binary "@/path/to/your/file" \
  https://file.oneonezero.dpdns.org/api/v1/files/upload
```

---

## 2. 下载文件

此端点用于下载已上传的文件。

- **URL**: `/api/v1/files/public/download/<FILE_ID>`
- **方法**: `GET`

### URL 参数 (URL Parameters)

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `FILE_ID` | string | 是 | 要下载的文件的 ID。 |

### 成功响应

  直接返回文件内容。

### cURL 示例

```bash
curl -X GET \
  -o "downloaded_file" \
  https://file.oneonezero.dpdns.org/api/v1/files/public/download/<FILE_ID>
```

---

## 3. 删除文件

此端点用于删除已上传的文件。

- **URL**: `/api/v1/files/delete/<FILE_ID>`
- **方法**: `DELETE`
- **认证**: `X-API-KEY: <YOUR_API_KEY>`

### URL 参数 (URL Parameters)

| 参数名 | 类型 | 是否必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `FILE_ID` | string | 是 | 要删除的文件的 ID。 |

### 成功响应

  **JSON 对象**: 包含删除成功的信息。
    - **Content-Type**: `application/json`
    ```json
    {
      "message": "File deleted successfully.",
      "ok": true
    }
    ```

### cURL 示例

```bash
curl -X DELETE \
  -H "X-API-KEY: <YOUR_API_KEY>" \
  https://file.oneonezero.dpdns.org/api/v1/files/delete/<FILE_ID>