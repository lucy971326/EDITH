# 构建响应 Webhook 事件的GitHub应用

了解如何构建 GitHub App，用于发出 API 请求以响应 Webhook 事件。

## 简介

本教程演示如何编写代码来创建 GitHub App，用于发出 API 请求以响应 Webhook 事件。 具体而言，在授予应用访问权限的存储库中打开拉取请求时，应用将收到拉取请求 Webhook 事件。 然后，应用将使用 GitHub 的 API 向拉取请求添加注释。

在本教程中，你将在开发应用时使用计算机或 codespace 作为服务器。 应用可供生产使用后，应将应用部署到专用服务器。

本教程使用 JavaScript，但你可以使用可在服务器上运行的任何编程语言。

### 关于 web 挂钩

注册 GitHub App 时，可以指定 Webhook URL 并订阅 Webhook 事件。 当 GitHub 上的活动触发应用订阅的事件时，GitHub 会将 Webhook 事件发送到应用的 Webhook URL。

例如，你可以为 GitHub App 订阅拉取请求 Webhook 事件。 在授予应用访问权限的存储库中打开拉取请求时，GitHub 会将拉取请求 Webhook 事件发送到应用的 Webhook URL。 如果多个操作可以触发该事件，则事件负载将包含一个 `action` 字段，用于指示触发事件的操作类型。 在此示例中，`action` 的值将为 `opened`，因为打开了拉取请求而触发了事件。

如果应用在侦听这些 Webhook 事件的服务器上运行，则应用可以在收到 Webhook 事件时执行操作。 例如，当应用收到拉取请求 Webhook 事件时，应用可以使用 GitHub API 向拉取请求发布注释。

有关详细信息，请参阅“[将 Webhook 与 GitHub 应用配合使用](/zh/apps/creating-github-apps/creating-github-apps/using-webhooks-with-github-apps)”。 有关可能的 Webhook 事件和操作的信息，请参阅 [Webhook 事件和有效负载](/zh/webhooks-and-events/webhooks/webhook-events-and-payloads)。

## Prerequisites

本教程要求计算机或 codespace 运行 Node.js 版本 20 或更高版本和 npm 版本 6.12.0 或更高版本。 有关详细信息，请参阅 [Node.js](https://nodejs.org)。

本教程假定你已基本了解 JavaScript 和 ES6 语法。

## 设置

以下部分将引导你设置以下组成部分：

* 用于存储应用代码的存储库
* 用于在本地接收 Webhook 的方式
* 订阅“拉取请求”Webhook 事件的 GitHub App 注册，有权向拉取请求添加注释，并使用可在本地接收的 Webhook URL

### 创建存储库来存储应用的代码

1. 创建存储库来存储应用的代码。 有关详细信息，请参阅“[创建新仓库](/zh/repositories/creating-and-managing-repositories/creating-a-new-repository)”。
2. 克隆上一步中的存储库。 有关详细信息，请参阅“[克隆仓库](/zh/repositories/creating-and-managing-repositories/cloning-a-repository)”。 可以使用本地克隆或 GitHub Codespaces。
3. 在终端中，导航到存储克隆的目录。
4. 如果目录尚未包含 `.gitignore` 文件，请添加 `.gitignore` 文件。 稍后会向此文件添加内容。 有关 `.gitignore` 文件的详细信息，请参阅“[忽略文件](/zh/get-started/git-basics/ignoring-files)”。

你将在后面的步骤中向此存储库添加更多代码。

### 获取 Webhook 代理 URL

若要在本地开发应用，可以使用 Webhook 代理 URL 将 Webhook 从 GitHub 转发到你的计算机或 codespace。 本教程使用 Smee.io 提供 Webhook 代理 URL 和转发 Webhook。

1. 在浏览器中，导航到  <https://smee.io/> 。
2. 单击**启动新频道**。
3. 复制“Webhook 代理 URL”下的完整 URL。 你将在后面的步骤中使用此 URL。

### 注册 GitHub App

对于本教程，你必须有一个满足以下条件的 GitHub App 注册：

* Webhook 处于活动状态
* 使用可在本地接收的 Webhook URL
* 具有“拉取请求”存储库权限
* 订阅“拉取请求”Webhook 事件

以下步骤将引导你使用这些设置来注册 GitHub App。 有关 GitHub App 设置的详细信息，请参阅 [注册GitHub应用](/zh/apps/creating-github-apps/creating-github-apps/creating-a-github-app)。

1. 在 GitHub 上任意页的右上角，单击你的个人资料图片。
2. 导航到你的帐户设置。
   * 对于由个人帐户拥有的应用，请单击“设置”\*\*\*\*。
   * 对于组织拥有的应用：
     1. 单击“你的组织”。
     2. 在组织的右侧，单击**设置**。
3. 在左边栏中，单击 <svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-code" aria-label="code" role="img"><path d="m11.28 3.22 4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.275-.326.749.749 0 0 1 .215-.734L13.94 8l-3.72-3.72a.749.749 0 0 1 .326-1.275.749.749 0 0 1 .734.215Zm-6.56 0a.751.751 0 0 1 1.042.018.751.751 0 0 1 .018 1.042L2.06 8l3.72 3.72a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215L.47 8.53a.75.75 0 0 1 0-1.06Z"></path></svg>“Developer settings”\*\*\*\*。
4. 在左侧边栏中，单击“GitHub Apps”。
5. 单击**新建 GitHub App**。
6. 在“GitHub App 名称”下面，输入应用的名称。 例如 `USERNAME-webhook-test-app`，其中 `USERNAME` 是 GitHub 用户名。
7. 在“主页 URL”下，输入应用的 URL。 例如，可以使用创建用于存储应用代码的存储库的 URL。
8. 跳过本教程的“标识和授权用户”和“安装后”部分。 有关这些设置的详细信息，请参阅 [注册GitHub应用](/zh/apps/creating-github-apps/creating-github-apps/creating-a-github-app)。
9. 确保在“Webhook”下选择**活动**。
10. 在“Webhook URL”下，输入前面提到的 Webhook 代理 URL。 有关详细信息，请参阅[获取 Webhook 代理 URL](#get-a-webhook-proxy-url)。
11. 在“Webhook 机密”下，输入一个随机字符串。 稍后会用到此字符串。
12. 在“存储库权限”下的“拉取请求”旁边，选择**读取和写入**。
13. 在“订阅事件”下面，选择**拉取请求**。
14. 在“此 GitHub App 可以安装在哪里？”下面，选择**仅在此帐户上**。 如果以后想发布你的应用，可以更改此设置。
15. 单击**创建 GitHub App**。

## 为应用编写代码

以下部分将引导你编写代码，使应用响应 Webhook 事件。

### 安装依赖项

本教程使用 GitHub 的 `octokit` 模块来处理 Webhook 事件并发出 API 请求。 有关 Octokit.js 的详细信息，请参阅 [使用 REST API 和 JavaScript 编写脚本](/zh/rest/guides/scripting-with-the-rest-api-and-javascript) 和 [Octokit.js README](https://github.com/octokit/octokit.js/#readme)。

本教程使用 `dotenv` 模块从 `.env` 文件中读取有关应用的信息。 有关详细信息，请参阅 [dotenv](https://www.npmjs.com/package/dotenv)。

本教程使用 Smee.io 将 Webhook 从 GitHub 转发到本地服务器。 有关详细信息，请参阅 [smee-client](https://www.npmjs.com/package/smee-client)。

1. 在终端中，导航到存储克隆的目录。
2. 运行 `npm init --yes` 以使用 npm 默认值创建 `package.json` 文件。
3. 运行 `npm install octokit`。
4. 运行 `npm install dotenv`。
5. 运行 `npm install smee-client --save-dev`。 由于在开发应用时仅使用 Smee.io 转发 Webhook，因此这是一个开发依赖项。
6. 将 `node_modules` 添加到 `.gitignore` 文件。

### 存储应用的标识信息和凭据

本教程介绍如何将应用的凭据和标识信息存储为 `.env` 文件中的环境变量。 部署应用时，需要更改凭据的存储方式。 有关详细信息，请参阅[部署你的应用](#deploy-your-app)。

在执行这些步骤之前，请确保你在使用安全的计算机，因为你将在本地存储凭据。

1. 在终端中，导航到存储克隆的目录。

2. 在此目录的顶级创建名为 `.env` 的文件。

3. 将 `.env` 添加到 `.gitignore` 文件。 这可以防止意外提交应用的凭据。

4. 将以下内容添加到 `.env` 文件。 将在后面的步骤中更新值。

   ```text copy
   APP_ID="YOUR_APP_ID"
   WEBHOOK_SECRET="YOUR_WEBHOOK_SECRET"
   PRIVATE_KEY_PATH="YOUR_PRIVATE_KEY_PATH"
   ```

5. 导航到应用的设置页面：

   1. 在 GitHub 上任意页的右上角，单击你的个人资料图片。

   2. 导航到你的帐户设置。
      * 对于由个人帐户拥有的应用，请单击“设置”\*\*\*\*。
      * 对于组织拥有的应用：
        1. 单击“你的组织”。
        2. 在组织的右侧，单击**设置**。

   3. 在左边栏中，单击 <svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-code" aria-label="code" role="img"><path d="m11.28 3.22 4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.275-.326.749.749 0 0 1 .215-.734L13.94 8l-3.72-3.72a.749.749 0 0 1 .326-1.275.749.749 0 0 1 .734.215Zm-6.56 0a.751.751 0 0 1 1.042.018.751.751 0 0 1 .018 1.042L2.06 8l3.72 3.72a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215L.47 8.53a.75.75 0 0 1 0-1.06Z"></path></svg>“Developer settings”\*\*\*\*。

   4. 在左侧边栏中，单击“GitHub Apps”。

   5. 在应用名称旁边，单击“编辑”。

6. 在应用的设置页上，在“应用 ID”旁边，找到应用的应用 ID。

7. 在 `.env` 文件中，将 `YOUR_APP_ID` 替换为应用的应用 ID。

8. 在应用的设置页上，在“私钥”下，单击**生成私钥**。 您将看到一个以 PEM 格式下载至您的计算机的私钥。 有关详细信息，请参阅“[管理GitHub应用的私钥](/zh/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)”。

9. 如果使用的是 codespace，请将下载的 PEM 文件移到 codespace 中，以便 codespace 可以访问该文件。

10. 在 `.env` 文件中，将 `YOUR_PRIVATE_KEY_PATH` 替换为私钥的完整路径，包括 `.pem` 扩展名。

11. 在 `.env` 文件中，将 `YOUR_WEBHOOK_SECRET` 替换为应用的 Webhook 机密。 如果忘记了 Webhook 机密，请在“Webhook 机密（可选）”下面单击**更改机密**。 输入新机密，然后单击**保存更改**。

### 添加代码以响应 Webhook 事件

在存储克隆的目录的顶级，创建一个 JavaScript 文件来保存应用的代码。 本教程将该文件命名为 `app.js`。

将以下代码添加到 `app.js`。 代码包含解释每个部分的注释。

```javascript copy annotate
// These are the dependencies for this file.
//
// You installed the `dotenv` and `octokit` modules earlier. The `@octokit/webhooks` is a dependency of the `octokit` module, so you don't need to install it separately. The `fs` and `http` dependencies are built-in Node.js modules.
import dotenv from "dotenv";
import {App from "octokit";
import {createNodeMiddleware} from "@octokit/webhooks";
import fs from "fs";
import http from "http";

// This reads your `.env` file and adds the variables from that file to the `process.env` object in Node.js.
dotenv.config();

// This assigns the values of your environment variables to local variables.
const appId = process.env.APP_ID;
const webhookSecret = process.env.WEBHOOK_SECRET;
const privateKeyPath = process.env.PRIVATE_KEY_PATH;

// This reads the contents of your private key file.
const privateKey = fs.readFileSync(privateKeyPath, "utf8");

// This creates a new instance of the Octokit App class.
const app = new App({
  appId: appId,
  privateKey: privateKey,
  webhooks: {
    secret: webhookSecret
  },
});

// This defines the message that your app will post to pull requests.
const messageForNewPRs = "Thanks for opening a new PR! Please follow our contributing guidelines to make your PR easier to review.";

// This adds an event handler that your code will call later. When this event handler is called, it will log the event to the console. Then, it will use GitHub's REST API to add a comment to the pull request that triggered the event.
async function handlePullRequestOpened({octokit, payload}) {
  console.log(`Received a pull request event for #${payload.pull_request.number}`);

  try {
    await octokit.request("POST /repos/{owner}/{repo}/issues/{issue_number}/comments", {
      owner: payload.repository.owner.login,
      repo: payload.repository.name,
      issue_number: payload.pull_request.number,
      body: messageForNewPRs,
      headers: {
        "x-github-api-version": "2026-03-10",
      },
    });
  } catch (error) {
    if (error.response) {
      console.error(`Error! Status: ${error.response.status}. Message: ${error.response.data.message}`)
    }
    console.error(error)
  }
};

// This sets up a webhook event listener. When your app receives a webhook event from GitHub with a `X-GitHub-Event` header value of `pull_request` and an `action` payload value of `opened`, it calls the `handlePullRequestOpened` event handler that is defined above.
app.webhooks.on("pull_request.opened", handlePullRequestOpened);

// This logs any errors that occur.
app.webhooks.onError((error) => {
  if (error.name === "AggregateError") {
    console.error(`Error processing request: ${error.event}`);
  } else {
    console.error(error);
  }
});

// This determines where your server will listen.
//
// For local development, your server will listen to port 3000 on `localhost`. When you deploy your app, you will change these values. For more information, see [Deploy your app](#deploy-your-app).
const port = 3000;
const host = 'localhost';
const path = "/api/webhook";
const localWebhookUrl = `http://${host}:${port}${path}`;

// This sets up a middleware function to handle incoming webhook events.
//
// Octokit's `createNodeMiddleware` function takes care of generating this middleware function for you. The resulting middleware function will:
//
// - Check the signature of the incoming webhook event to make sure that it matches your webhook secret. This verifies that the incoming webhook event is a valid GitHub event.
// - Parse the webhook event payload and identify the type of event.
// - Trigger the corresponding webhook event handler.
const middleware = createNodeMiddleware(app.webhooks, {path});

// This creates a Node.js server that listens for incoming HTTP requests (including webhook payloads from GitHub) on the specified port. When the server receives a request, it executes the `middleware` function that you defined earlier. Once the server is running, it logs messages to the console to indicate that it is listening.
http.createServer(middleware).listen(port, () => {
  console.log(`Server is listening for events at: ${localWebhookUrl}`);
  console.log('Press Ctrl + C to quit.')
});
```

### 添加脚本以运行应用代码

1. 向 `scripts` 文件中的 `package.json` 对象添加名为 `server` 的脚本，该脚本运行 `node app.js`。 例如：

   ```json copy
   "scripts": {
     "server": "node app.js"
   }
   ```

   如果你把保存应用代码的文件命名为除 `app.js` 以外的名称，请将 `app.js` 替换为保存应用代码的文件的相对路径。

2. 在 `package.json` 文件中，添加一个值为 `type` 的顶级项 `module`。 例如：

   ```jsonc
      {
       // rest of the JSON object,
       "version": "1.0.0",
       "description": "",
       "type": "module",
       // rest of the JSON object,
     }
   ```

`package.json` 文件应如下所示。
`name` 和 `dependencies` 下的 `devDependencies` 值和版本号可能会有所不同。

```json
  {
  "name": "github-app-webhook-tutorial",
  "version": "1.0.0",
  "description": "",
  "main": "index.js",
  "type": "module",
  "scripts": {
    "server": "node app.js"
  },
  "keywords": [],
  "author": "",
  "license": "ISC",
  "dependencies": {
    "dotenv": "^16.0.3",
    "octokit": "^2.0.14"
  },
  "devDependencies": {
    "smee-client": "^1.2.3"
  }
}
```

## 测试

按照以下步骤测试上面创建的应用。

### 安装应用

为了使应用能够对存储库中的拉取请求留下注释，它必须安装在拥有该存储库的帐户上，并获得对该存储库的访问权限。 因为应用是私有的，因此只能在拥有该应用的帐户上进行安装。

1. 在拥有所创建应用的帐户中，创建一个新的存储库以安装应用。 有关详细信息，请参阅“[创建新仓库](/zh/repositories/creating-and-managing-repositories/creating-a-new-repository)”。
2. 导航到应用的设置页面：

   1. 在 GitHub 上任意页的右上角，单击你的个人资料图片。

   2. 导航到你的帐户设置。
      * 对于由个人帐户拥有的应用，请单击“设置”\*\*\*\*。
      * 对于组织拥有的应用：
        1. 单击“你的组织”。
        2. 在组织的右侧，单击**设置**。

   3. 在左边栏中，单击 <svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-code" aria-label="code" role="img"><path d="m11.28 3.22 4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.275-.326.749.749 0 0 1 .215-.734L13.94 8l-3.72-3.72a.749.749 0 0 1 .326-1.275.749.749 0 0 1 .734.215Zm-6.56 0a.751.751 0 0 1 1.042.018.751.751 0 0 1 .018 1.042L2.06 8l3.72 3.72a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215L.47 8.53a.75.75 0 0 1 0-1.06Z"></path></svg>“Developer settings”\*\*\*\*。

   4. 在左侧边栏中，单击“GitHub Apps”。

   5. 在应用名称旁边，单击“编辑”。
3. 单击**公共页面**。
4. 单击“安装” 。
5. 选择“仅选择存储库\*\*\*\*”。
6. 选择**选择存储库**下拉菜单，然后单击在本部分开头选择的存储库。
7. 单击“安装” 。

### 启动服务器

对于测试，你将使用计算机或 codespace 作为服务器。 应用仅在服务器运行时响应 Webhook。

1. 在终端中，导航到存储应用代码的目录。

2. 若要从 Smee.io 接收转发的 Webhook，请运行 `npx smee -u WEBHOOK_PROXY_URL -t http://localhost:3000/api/webhook`。 将 `WEBHOOK_PROXY_URL` 替换为前面提到的 Webhook 代理 URL。 如果忘记了 URL，可以在应用设置页上的“Webhook URL”字段中找到它。

   应会看到如下所示的输出，其中 `WEBHOOK_PROXY_URL` 是 Webhook 代理 URL：

   ```shell
   Forwarding WEBHOOK_PROXY_URL to http://localhost:3000/api/webhook
   Connected WEBHOOK_PROXY_URL
   ```

3. 在另一个终端窗口中，导航到存储应用代码的目录。

4. 运行 `npm run server`。 终端应显示 `Server is listening for events at: http://localhost:3000/api/webhook`。

### 测试应用

现在，服务器正在运行并接收转发的 Webhook 事件，请通过在安装应用时选择的存储库上打开一个拉取请求来测试应用。

1. 在安装应用时选择的存储库上打开拉取请求。 有关详细信息，请参阅“[创建拉取请求](/zh/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request)”。

   请确保使用安装应用时选择的存储库，而不是存储应用代码的存储库。 有关详细信息，请参阅[安装应用](#install-your-app)。

2. 在 smee.io 上导航到 Webhook 代理 URL。 应该会看到 `pull_request` 事件。 这表示 GitHub 在创建拉取请求时成功发送了拉取请求事件。

3. 在运行 `npm run server` 的终端中，应会看到类似“已收到 #1 的拉取请求事件”的内容，其中 `#` 后面的整数是打开的拉取请求的编号。

4. 在拉取请求的时间线中，应会看到来自应用的注释。

5. 在这两个终端窗口中，输入 <kbd>Ctrl</kbd>+<kbd>C</kbd> 以停止服务器并停止侦听转发的 Webhook。

## 后续步骤

现在，你已有一个可响应 Webhook 事件的应用，可能想要扩展应用的代码、部署应用并公开应用。

### 修改应用代码

本教程演示了如何在打开拉取请求时对拉取请求发布注释。 可以更新代码以响应不同类型的 Webhook 事件，或执行不同的操作来响应 Webhook 事件。

针对你要发出的 API 请求或希望接收的 Webhook 事件，如果应用需要其他权限，请记得更新应用的权限。 有关详细信息，请参阅“[为GitHub应用选择权限](/zh/apps/creating-github-apps/creating-github-apps/setting-permissions-for-github-apps)”。

本教程将所有代码存储在单个文件中，但你可能希望将函数和组件移动到单独的文件中。

### 部署应用

本教程演示了如何在本地开发应用。 准备好部署应用后，需要进行更改来为应用提供服务，并确保应用的凭据安全。 执行的步骤取决于所使用的服务器，但以下部分提供了一般指导。

#### 在服务器上托管你的应用

本教程使用了计算机或 codespace 作为服务器。 应用可供生产使用后，应将应用部署到专用服务器。 例如，可以使用 [Azure App Service](https://azure.microsoft.com/products/app-service/)。

#### 更新 Webhook URL

将服务器设置为从 GitHub 接收 Webhook 流量后，请在应用设置中更新 Webhook URL。 不应使用 Smee.io 在生产环境中转发 Webhook。

#### 更新 `port` 和 `host` 常量

部署应用时，需要更改服务器正在侦听的主机和端口。

例如，可以在服务器上设置 `PORT` 环境变量，以指示服务器应侦听的端口。 可以将服务器上的 `NODE_ENV` 环境变量设置为 `production`。 然后，可以更新代码定义 `port` 和 `host` 常量的位置，以便服务器在部署端口上侦听所有可用的网络接口 (`0.0.0.0`)，而不是本地网络接口 (`localhost`)：

```javascript copy
const port = process.env.PORT || 3000;
const host = process.env.NODE_ENV === 'production' ? '0.0.0.0' : 'localhost';
```

#### 保护应用的凭据

切勿公开应用的私钥或 Webhook 机密。 本教程将应用的凭据存储在 gitignored `.env` 文件中。 部署应用时，应选择一种安全的方式来存储凭据并更新代码以获取相应值。 例如，可以使用机密管理服务（如 [Azure Key Vault](https://azure.microsoft.com/en-us/products/key-vault)）存储凭据。 应用运行时，它可以检索凭据并将其存储在部署应用的服务器上的环境变量中。

有关详细信息，请参阅“[创建GitHub应用的最佳做法](/zh/apps/creating-github-apps/setting-up-a-github-app/best-practices-for-creating-a-github-app)”。

### 共享应用

如果要与其他用户和组织共享应用，请公开应用。 有关详细信息，请参阅“[将GitHub应用公开或专用](/zh/apps/creating-github-apps/creating-github-apps/making-a-github-app-public-or-private)”。

### 遵循最佳做法

你的目标应该是遵循 GitHub App 的最佳做法。 有关详细信息，请参阅“[创建GitHub应用的最佳做法](/zh/apps/creating-github-apps/setting-up-a-github-app/best-practices-for-creating-a-github-app)”。