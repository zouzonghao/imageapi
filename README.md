# ImageAPI Web UI

这是一个基于 Go 语言的 Web 应用，为多个 AI 图像生成服务提供了一个统一、简单易用的图形用户界面和外部 API。用户可以通过 Web 界面上传图片、调整参数，将图片转换为不同的艺术风格，也可以通过 API 进行集成。

## 功能

-   **多 Provider 支持**：集成了 Dreamifly, Fal.ai, ModelScope, Pollinations.ai 等多个图像生成服务。
-   **图片上传与预览**：支持选择本地图片文件或提供图片 URL。
-   **参数可调**：允许用户自定义提示词 (Prompt)、选择模型 (Model)、调整尺寸、步数 (Steps) 和种子 (Seed)。
-   **Web UI 访问控制**：可通过环境变量设置密码，保护 Web 界面的访问。
-   **外部 API**：提供基于 API Key 认证的外部接口，方便程序化调用和集成。
-   **简洁界面**：清晰直观的界面布局，易于上手。

## 技术栈

-   **后端**：Go (使用标准库 `net/http` 和 `gorilla/sessions`)
-   **前端**：HTML, CSS, JavaScript (无框架)

## 如何运行

### 前提条件

-   已安装 [Go](https://golang.org/dl/) (版本 1.18 或更高)

### 步骤

1.  **克隆或下载项目**
    将本项目代码下载到您的本地机器。

2.  **安装依赖**
    项目依赖 `gorilla/sessions`，首次运行时 Go 会自动下载。如果需要手动安装，可以运行：
    ```bash
    go get github.com/gorilla/sessions
    ```

3.  **配置环境变量**
    项目通过 `.env` 文件管理敏感信息和配置。请将 `.env.example` 文件复制一份并重命名为 `.env`：
    ```bash
    cp .env.example .env
    ```
    然后，编辑 `.env` 文件，填入必要的 API Keys 和自定义配置。

    **核心配置**:
    -   `NODEIMAGE_API_KEY`: 用于上传和托管图片的 [nodeimage.io](https://nodeimage.io/) 的 API Key。
    -   `WEB_PASSWORD`: 用于登录 Web 界面的密码。如果留空，Web 界面将无需登录即可访问。
    -   `IMAGEAPI_API_KEY`: 用于访问外部 API 的密钥。如果留空，外部 API 将被禁用。
    -   `SESSION_SECRET`: 用于加密 session cookie 的密钥，请设置为一个长且随机的字符串。
    -   `FAL_API_KEY`, `MODELSCOPE_API_KEY`, `POLLINATIONS_AI_API_KEY`: 各个 AI 服务提供商的 API Key，按需填写。

4.  **运行 Go 服务器**
    在项目根目录下，打开终端并执行以下命令：
    ```bash
    go run main.go
    ```

5.  **访问应用**
    服务器启动后，您会看到一条日志信息：
    ```
    Starting server on :8080...
    ```
    此时，打开您的浏览器并访问 `http://localhost:8080`。如果设置了 `WEB_PASSWORD`，您将被引导至登录页面。

## 外部 API 使用说明

项目提供了一套 v1 版本的 API，用于程序化地生成图片和查询模型。

**认证**: 所有 API 请求都需要在 HTTP Header 中提供 `Authorization` 字段进行认证。
-   **格式**: `Bearer <your_imageapi_api_key>`
-   **示例**: `Authorization: Bearer your_secret_api_key`

---

### 1. 获取可用模型

-   **URL**: `/api/v1/models`
-   **方法**: `GET`
-   **成功响应 (200 OK)**:
    ```json
    [
        {
            "provider": "Dreamifly",
            "models": [
                {
                    "name": "Flux-Kontext",
                    "supported_params": ["steps", "seed", "image"],
                    "max_width": 1920,
                    "max_height": 1920
                }
            ]
        }
    ]
    ```

---

### 2. 生成图片

-   **URL**: `/api/v1/generate`
-   **方法**: `POST`
-   **请求体 (JSON)**:
    ```json
    {
        "prompt": "a beautiful landscape painting",
        "model": "Dreamifly/Wai-SDXL-V150",
        "width": 1024,
        "height": 1024,
        "image_url": "https://example.com/optional_input_image.jpg",
        "seed": 12345,
        "steps": 30
    }
    ```
    -   `prompt` (string, 必填): 提示词。
    -   `model` (string, 必填): 模型名称，格式为 `provider_name/model_name`。
    -   `width`, `height` (int, 可选): 图片尺寸，默认为 1024x1024。
    -   `image_url` (string, 可选): 如果使用的模型支持图生图，提供输入图片的 URL。
    -   `seed`, `steps` (int, 可选): 其他生成参数。

-   **成功响应 (200 OK)**:
    ```json
    {
        "status": "success",
        "image_url": "https://img.nodeimage.io/..."
    }
    ```

-   **失败响应 (4xx/5xx)**:
    ```json
    {
        "status": "error",
        "error": "Error message details..."
    }
    ```

---

**cURL 示例**:

```bash
curl -X POST http://localhost:8080/api/v1/generate \
-H "Authorization: Bearer your_secret_api_key" \
-H "Content-Type: application/json" \
-d '{
    "prompt": "a cute cat in a space suit, high quality",
    "model": "Dreamifly/Wai-SDXL-V150",
    "width": 1024,
    "height": 1024
}'
```

---

### 3. 生成图片 (图生图示例)

-   **URL**: `/api/v1/generate`
-   **方法**: `POST`
-   **请求体 (JSON)**:
    ```json
    {
        "prompt": "make it winter",
        "model": "Dreamifly/Flux-Kontext",
        "width": 1024,
        "height": 1024,
        "image_url": "https://img.nodeimage.io/user/1/upload/2024/09/some-image.jpg"
    }
    ```
    -   **注意**: `model` 必须选择支持图生图的模型 (例如 `Dreamifly/Flux-Kontext` 或 `modelscope/Qwen-Image-Edit`)，并且必须提供 `image_url`。
